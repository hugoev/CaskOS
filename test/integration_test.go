package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/caskos/caskos/internal/api"
	"github.com/caskos/caskos/internal/hashring"
	"github.com/caskos/caskos/internal/metadata"
	"github.com/caskos/caskos/internal/storage"
	"log/slog"
)

func TestUploadDownloadRoundTrip(t *testing.T) {
	// Setup test environment
	tmpDataDir, _ := os.MkdirTemp("", "caskos-data")
	tmpMetaDir, _ := os.MkdirTemp("", "caskos-meta")
	defer os.RemoveAll(tmpDataDir)
	defer os.RemoveAll(tmpMetaDir)

	// Create metadata store
	metaStore, err := metadata.NewStore(tmpMetaDir)
	if err != nil {
		t.Fatalf("failed to create metadata store: %v", err)
	}

	// Create hash ring
	ring := hashring.NewHashRing(3)

	// Create storage manager
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	storageManager := storage.NewManager(ring, 2, logger)

	// Create storage nodes
	for i := 1; i <= 3; i++ {
		nodeID := fmt.Sprintf("node%d", i)
		nodePath := filepath.Join(tmpDataDir, nodeID)
		node, err := storage.NewNode(nodeID, nodePath)
		if err != nil {
			t.Fatalf("failed to create node: %v", err)
		}
		ring.AddNode(nodeID)
		storageManager.AddNode(nodeID, node)
	}

	// Create API server
	server := api.NewServer(storageManager, metaStore, logger, 2)

	// Test data
	testData := "This is test file content for upload/download test"
	testFileName := "test.txt"

	// Create multipart form data
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	part, err := writer.CreateFormFile("file", testFileName)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	part.Write([]byte(testData))
	writer.Close()

	// Create upload request
	uploadReq := httptest.NewRequest(http.MethodPost, "/upload", &requestBody)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRecorder := httptest.NewRecorder()

	// Execute upload
	server.UploadHandler(uploadRecorder, uploadReq)

	// Check response
	if uploadRecorder.Code != http.StatusCreated && uploadRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 201 or 200, got %d: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}

	// Parse response to get object ID
	var uploadResponse map[string]interface{}
	if err := json.Unmarshal(uploadRecorder.Body.Bytes(), &uploadResponse); err != nil {
		t.Fatalf("failed to parse upload response: %v", err)
	}

	objectID, ok := uploadResponse["id"].(string)
	if !ok {
		t.Fatalf("object ID not found in response")
	}

	// Retrieve object - need to set path value manually for direct handler calls
	getReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/object/%s", objectID), nil)
	getReq.SetPathValue("id", objectID)
	getRecorder := httptest.NewRecorder()

	server.GetObjectHandler(getRecorder, getReq)

	// Check response
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", getRecorder.Code, getRecorder.Body.String())
	}

	// Verify data
	retrievedData := getRecorder.Body.String()
	if retrievedData != testData {
		t.Errorf("data mismatch: expected %q, got %q", testData, retrievedData)
	}

	// Test metadata endpoint
	metaReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/metadata/%s", objectID), nil)
	metaReq.SetPathValue("id", objectID)
	metaRecorder := httptest.NewRecorder()

	server.GetMetadataHandler(metaRecorder, metaReq)

	if metaRecorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", metaRecorder.Code)
	}

	var metaResponse map[string]interface{}
	if err := json.Unmarshal(metaRecorder.Body.Bytes(), &metaResponse); err != nil {
		t.Fatalf("failed to parse metadata response: %v", err)
	}

	if metaResponse["id"] != objectID {
		t.Errorf("metadata ID mismatch: expected %s, got %v", objectID, metaResponse["id"])
	}
}

