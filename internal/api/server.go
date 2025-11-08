package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/caskos/caskos/internal/metadata"
	"github.com/caskos/caskos/internal/storage"
)

// Server handles HTTP requests for the object storage API
type Server struct {
	storageManager *storage.Manager
	metadataStore  *metadata.Store
	logger         *slog.Logger
	replication    int
}

// NewServer creates a new API server
func NewServer(
	storageManager *storage.Manager,
	metadataStore *metadata.Store,
	logger *slog.Logger,
	replication int,
) *Server {
	return &Server{
		storageManager: storageManager,
		metadataStore:  metadataStore,
		logger:         logger,
		replication:    replication,
	}
}

// UploadHandler handles object uploads
func (s *Server) UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse multipart form (max 100MB)
	if err := r.ParseMultipartForm(100 << 20); err != nil {
		s.logger.Error("failed to parse multipart form", "error", err)
		http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.logger.Error("failed to get file from form", "error", err)
		http.Error(w, fmt.Sprintf("Failed to get file: %v", err), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file data
	data, err := io.ReadAll(file)
	if err != nil {
		s.logger.Error("failed to read file data", "error", err)
		http.Error(w, fmt.Sprintf("Failed to read file: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate object ID from content hash
	objectID := storage.GenerateObjectID(data)

	// Check if object already exists
	if s.metadataStore.Exists(objectID) {
		existingMeta, err := s.metadataStore.Get(objectID)
		if err == nil {
			s.respondWithMetadata(w, existingMeta, http.StatusOK)
			return
		}
	}

	// Store object with replication
	replicatedNodes, err := s.storageManager.StoreObject(objectID, io.NopCloser(io.NewSectionReader(
		&byteReader{data: data}, 0, int64(len(data)),
	)), int64(len(data)))
	if err != nil {
		s.logger.Error("failed to store object", "error", err, "object_id", objectID)
		http.Error(w, fmt.Sprintf("Failed to store object: %v", err), http.StatusInternalServerError)
		return
	}

	// Create metadata
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	meta := &metadata.ObjectMetadata{
		ID:          objectID,
		Size:        int64(len(data)),
		ContentType: contentType,
		CreatedAt:   time.Now(),
		Replicas:    replicatedNodes,
	}

	// Save metadata
	if err := s.metadataStore.Save(meta); err != nil {
		s.logger.Error("failed to save metadata", "error", err, "object_id", objectID)
		// Object is stored but metadata failed - this is a problem but we'll continue
	}

	// Check for missing replicas and trigger self-healing
	go s.ensureReplication(objectID, meta)

	s.respondWithMetadata(w, meta, http.StatusCreated)
}

// GetObjectHandler retrieves an object
func (s *Server) GetObjectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	objectID := r.PathValue("id")
	if objectID == "" {
		http.Error(w, "Object ID is required", http.StatusBadRequest)
		return
	}

	// Retrieve object
	reader, err := s.storageManager.RetrieveObject(objectID)
	if err != nil {
		s.logger.Warn("object not found", "object_id", objectID, "error", err)
		http.Error(w, "Object not found", http.StatusNotFound)
		return
	}
	defer reader.Close()

	// Get metadata for content type
	meta, err := s.metadataStore.Get(objectID)
	if err == nil && meta.ContentType != "" {
		w.Header().Set("Content-Type", meta.ContentType)
	}

	// Stream object data
	if _, err := io.Copy(w, reader); err != nil {
		s.logger.Error("failed to stream object", "error", err, "object_id", objectID)
		return
	}
}

// GetMetadataHandler retrieves object metadata
func (s *Server) GetMetadataHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	objectID := r.PathValue("id")
	if objectID == "" {
		http.Error(w, "Object ID is required", http.StatusBadRequest)
		return
	}

	meta, err := s.metadataStore.Get(objectID)
	if err != nil {
		s.logger.Warn("metadata not found", "object_id", objectID, "error", err)
		http.Error(w, "Metadata not found", http.StatusNotFound)
		return
	}

	// Update replica status
	availableReplicas := s.storageManager.CheckReplicas(objectID)
	meta.Replicas = availableReplicas

	// Trigger self-healing if needed
	if len(availableReplicas) < s.replication {
		go s.ensureReplication(objectID, meta)
	}

	s.respondWithMetadata(w, meta, http.StatusOK)
}

// ensureReplication ensures an object has the required number of replicas
func (s *Server) ensureReplication(objectID string, meta *metadata.ObjectMetadata) {
	availableReplicas := s.storageManager.CheckReplicas(objectID)
	if len(availableReplicas) >= s.replication {
		return // Already have enough replicas
	}

	s.logger.Info("detected missing replicas, starting self-healing",
		"object_id", objectID,
		"current_replicas", len(availableReplicas),
		"required", s.replication)

	// Get target nodes from hash ring
	targetNodes := s.storageManager.GetTargetNodes(objectID)

	// Create a set of available replicas for quick lookup
	availableSet := make(map[string]bool)
	for _, nodeID := range availableReplicas {
		availableSet[nodeID] = true
	}

	// Replicate to nodes that should have it but don't
	replicated := 0
	for _, targetNodeID := range targetNodes {
		if availableSet[targetNodeID] {
			continue // Already has the object
		}

		// Replicate to this node (ReplicateObject will retrieve the object internally)
		if err := s.storageManager.ReplicateObject(objectID, targetNodeID); err != nil {
			s.logger.Error("failed to replicate object to node",
				"error", err,
				"object_id", objectID,
				"target_node", targetNodeID)
			continue
		}

		replicated++
		s.logger.Info("replicated object to node",
			"object_id", objectID,
			"node_id", targetNodeID)

		if len(availableReplicas)+replicated >= s.replication {
			break // We have enough replicas now
		}
	}

	// Update metadata with new replica list
	if replicated > 0 {
		updatedReplicas := s.storageManager.CheckReplicas(objectID)
		meta.Replicas = updatedReplicas
		if err := s.metadataStore.Save(meta); err != nil {
			s.logger.Error("failed to update metadata after replication", "error", err, "object_id", objectID)
		}
	}
}

// respondWithMetadata sends metadata as JSON response
func (s *Server) respondWithMetadata(w http.ResponseWriter, meta *metadata.ObjectMetadata, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := map[string]interface{}{
		"id":           meta.ID,
		"size":         meta.Size,
		"content_type": meta.ContentType,
		"created_at":   meta.CreatedAt.Format(time.RFC3339),
		"replicas":    meta.Replicas,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode response", "error", err)
	}
}

// byteReader implements io.ReaderAt for byte slices
type byteReader struct {
	data []byte
}

func (br *byteReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, fmt.Errorf("negative offset")
	}
	if off >= int64(len(br.data)) {
		return 0, io.EOF
	}
	n = copy(p, br.data[off:])
	if n < len(p) {
		err = io.EOF
	}
	return n, err
}

