package storage

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/caskos/caskos/internal/hashring"
	"log/slog"
)

func TestManager_StoreAndRetrieve(t *testing.T) {
	// Create temporary directories
	tmpDir1, _ := os.MkdirTemp("", "storage-node1")
	tmpDir2, _ := os.MkdirTemp("", "storage-node2")
	defer os.RemoveAll(tmpDir1)
	defer os.RemoveAll(tmpDir2)

	// Create hash ring and manager
	ring := hashring.NewHashRing(3)
	ring.AddNode("node1")
	ring.AddNode("node2")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(ring, 2, logger)

	// Create nodes
	node1, _ := NewNode("node1", tmpDir1)
	node2, _ := NewNode("node2", tmpDir2)
	manager.AddNode("node1", node1)
	manager.AddNode("node2", node2)

	// Generate object ID
	testData := "This is test data for storage manager"
	objectID := GenerateObjectID([]byte(testData))

	// Store object
	reader := strings.NewReader(testData)
	replicatedNodes, err := manager.StoreObject(objectID, reader, int64(len(testData)))
	if err != nil {
		t.Fatalf("failed to store object: %v", err)
	}

	if len(replicatedNodes) == 0 {
		t.Error("expected at least one replica")
	}

	// Retrieve object
	retrieved, err := manager.RetrieveObject(objectID)
	if err != nil {
		t.Fatalf("failed to retrieve object: %v", err)
	}
	defer retrieved.Close()

	// Verify data
	data, err := io.ReadAll(retrieved)
	if err != nil {
		t.Fatalf("failed to read retrieved data: %v", err)
	}

	if string(data) != testData {
		t.Errorf("data mismatch: expected %q, got %q", testData, string(data))
	}
}

func TestManager_GenerateObjectID(t *testing.T) {
	testData := "test data"
	objectID1 := GenerateObjectID([]byte(testData))
	objectID2 := GenerateObjectID([]byte(testData))

	// Same data should generate same ID
	if objectID1 != objectID2 {
		t.Errorf("expected same object ID for same data, got %s and %s", objectID1, objectID2)
	}

	// Different data should generate different ID
	otherData := "different data"
	objectID3 := GenerateObjectID([]byte(otherData))
	if objectID1 == objectID3 {
		t.Error("expected different object IDs for different data")
	}

	// Object ID should be 64 characters (SHA256 hex)
	if len(objectID1) != 64 {
		t.Errorf("expected object ID length 64, got %d", len(objectID1))
	}
}

func TestManager_CheckReplicas(t *testing.T) {
	tmpDir1, _ := os.MkdirTemp("", "storage-node1")
	tmpDir2, _ := os.MkdirTemp("", "storage-node2")
	defer os.RemoveAll(tmpDir1)
	defer os.RemoveAll(tmpDir2)

	ring := hashring.NewHashRing(3)
	ring.AddNode("node1")
	ring.AddNode("node2")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	manager := NewManager(ring, 2, logger)

	node1, _ := NewNode("node1", tmpDir1)
	node2, _ := NewNode("node2", tmpDir2)
	manager.AddNode("node1", node1)
	manager.AddNode("node2", node2)

	testData := "test data"
	objectID := GenerateObjectID([]byte(testData))

	reader := strings.NewReader(testData)
	_, err := manager.StoreObject(objectID, reader, int64(len(testData)))
	if err != nil {
		t.Fatalf("failed to store object: %v", err)
	}

	replicas := manager.CheckReplicas(objectID)
	if len(replicas) == 0 {
		t.Error("expected at least one replica")
	}
}

