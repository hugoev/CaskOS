package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

// Manager coordinates storage across multiple nodes with replication
type Manager struct {
	mu          sync.RWMutex
	nodes       map[string]*Node
	hashRing    HashRingInterface
	replication int
	logger      *slog.Logger
}

// HashRingInterface defines the interface for hash ring operations
type HashRingInterface interface {
	GetNodes(key string, count int) []string
	ListNodes() []string
	NodeCount() int
}

// NewManager creates a new storage manager
func NewManager(hashRing HashRingInterface, replication int, logger *slog.Logger) *Manager {
	return &Manager{
		nodes:       make(map[string]*Node),
		hashRing:    hashRing,
		replication: replication,
		logger:      logger,
	}
}

// AddNode adds a storage node to the manager
func (m *Manager) AddNode(nodeID string, node *Node) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nodes[nodeID] = node
}

// StoreObject stores an object with replication
func (m *Manager) StoreObject(objectID string, data io.Reader, size int64) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get nodes for this object using consistent hashing
	targetNodes := m.hashRing.GetNodes(objectID, m.replication)
	if len(targetNodes) == 0 {
		return nil, fmt.Errorf("no storage nodes available")
	}

	// Read data into memory for replication (for small to medium files)
	// For large files, we'd want to stream to multiple nodes, but for simplicity
	// we'll read into memory first
	dataBytes, err := io.ReadAll(data)
	if err != nil {
		return nil, fmt.Errorf("failed to read object data: %w", err)
	}

	replicatedNodes := make([]string, 0, len(targetNodes))
	var lastErr error

	// Store on multiple nodes
	for _, nodeID := range targetNodes {
		node, exists := m.nodes[nodeID]
		if !exists {
			m.logger.Warn("node not found in manager", "node_id", nodeID)
			continue
		}

		// Create a new reader from the bytes for each node
		reader := io.NopCloser(io.NewSectionReader(
			&byteReader{data: dataBytes}, 0, int64(len(dataBytes)),
		))

		if err := node.Store(objectID, reader); err != nil {
			m.logger.Error("failed to store object on node", "node_id", nodeID, "error", err)
			lastErr = err
			continue
		}

		replicatedNodes = append(replicatedNodes, nodeID)
		m.logger.Info("stored object on node", "object_id", objectID, "node_id", nodeID)
	}

	if len(replicatedNodes) == 0 {
		return nil, fmt.Errorf("failed to store object on any node: %w", lastErr)
	}

	return replicatedNodes, nil
}

// RetrieveObject retrieves an object from any available replica
func (m *Manager) RetrieveObject(objectID string) (io.ReadCloser, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get nodes that should have this object
	targetNodes := m.hashRing.GetNodes(objectID, m.replication)

	// Try each node until we find one with the object
	for _, nodeID := range targetNodes {
		node, exists := m.nodes[nodeID]
		if !exists {
			continue
		}

		if !node.Exists(objectID) {
			continue
		}

		reader, err := node.Retrieve(objectID)
		if err == nil {
			m.logger.Info("retrieved object from node", "object_id", objectID, "node_id", nodeID)
			return reader, nil
		}
	}

	return nil, fmt.Errorf("object not found on any available node: %s", objectID)
}

// ReplicateObject replicates an object to a specific node (for self-healing)
func (m *Manager) ReplicateObject(objectID string, targetNodeID string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// First, find the object on any existing node
	sourceReader, err := m.RetrieveObject(objectID)
	if err != nil {
		return fmt.Errorf("failed to retrieve object for replication: %w", err)
	}
	defer sourceReader.Close()

	// Get the target node
	targetNode, exists := m.nodes[targetNodeID]
	if !exists {
		return fmt.Errorf("target node not found: %s", targetNodeID)
	}

	// Store on target node
	if err := targetNode.Store(objectID, sourceReader); err != nil {
		return fmt.Errorf("failed to replicate object to node: %w", err)
	}

	m.logger.Info("replicated object to node", "object_id", objectID, "node_id", targetNodeID)
	return nil
}

// CheckReplicas checks which nodes have replicas of an object
func (m *Manager) CheckReplicas(objectID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	availableNodes := make([]string, 0)
	for nodeID, node := range m.nodes {
		if node.Exists(objectID) {
			availableNodes = append(availableNodes, nodeID)
		}
	}

	return availableNodes
}

// GetTargetNodes returns the nodes that should store an object according to the hash ring
func (m *Manager) GetTargetNodes(objectID string) []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.hashRing.GetNodes(objectID, m.replication)
}

// GenerateObjectID generates a unique object ID from data
func GenerateObjectID(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
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
