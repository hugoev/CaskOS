package storage

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNode_StoreAndRetrieve(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	node, err := NewNode("test-node", tmpDir)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	objectID := "abcdef1234567890abcdef1234567890"
	testData := "Hello, World! This is test data."

	// Store object
	reader := strings.NewReader(testData)
	if err := node.Store(objectID, reader); err != nil {
		t.Fatalf("failed to store object: %v", err)
	}

	// Verify object exists
	if !node.Exists(objectID) {
		t.Error("expected object to exist after store")
	}

	// Retrieve object
	retrieved, err := node.Retrieve(objectID)
	if err != nil {
		t.Fatalf("failed to retrieve object: %v", err)
	}
	defer retrieved.Close()

	// Read and verify data
	data, err := io.ReadAll(retrieved)
	if err != nil {
		t.Fatalf("failed to read retrieved data: %v", err)
	}

	if string(data) != testData {
		t.Errorf("data mismatch: expected %q, got %q", testData, string(data))
	}
}

func TestNode_GetSize(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	node, err := NewNode("test-node", tmpDir)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	objectID := "abcdef1234567890abcdef1234567890"
	testData := "Test data for size check"

	reader := strings.NewReader(testData)
	if err := node.Store(objectID, reader); err != nil {
		t.Fatalf("failed to store object: %v", err)
	}

	size, err := node.GetSize(objectID)
	if err != nil {
		t.Fatalf("failed to get size: %v", err)
	}

	expectedSize := int64(len(testData))
	if size != expectedSize {
		t.Errorf("size mismatch: expected %d, got %d", expectedSize, size)
	}
}

func TestNode_Delete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	node, err := NewNode("test-node", tmpDir)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	objectID := "abcdef1234567890abcdef1234567890"
	testData := "Test data"

	reader := strings.NewReader(testData)
	if err := node.Store(objectID, reader); err != nil {
		t.Fatalf("failed to store object: %v", err)
	}

	if err := node.Delete(objectID); err != nil {
		t.Fatalf("failed to delete object: %v", err)
	}

	if node.Exists(objectID) {
		t.Error("expected object to not exist after delete")
	}
}

func TestNode_DirectoryStructure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "storage-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	node, err := NewNode("test-node", tmpDir)
	if err != nil {
		t.Fatalf("failed to create node: %v", err)
	}

	objectID := "abcdef1234567890abcdef1234567890"
	testData := "Test data"

	reader := strings.NewReader(testData)
	if err := node.Store(objectID, reader); err != nil {
		t.Fatalf("failed to store object: %v", err)
	}

	// Verify directory structure: basePath/ab/cd/objectID
	expectedPath := filepath.Join(tmpDir, "ab", "cd", objectID)
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("expected object file at %s, but got error: %v", expectedPath, err)
	}
}

