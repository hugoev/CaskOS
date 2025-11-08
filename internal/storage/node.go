package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Node represents a storage node (a directory on disk)
type Node struct {
	ID       string
	BasePath string
	mu       sync.RWMutex
}

// NewNode creates a new storage node
func NewNode(id, basePath string) (*Node, error) {
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage node directory: %w", err)
	}

	return &Node{
		ID:       id,
		BasePath: basePath,
	}, nil
}

// Store writes object data to the storage node
func (n *Node) Store(objectID string, data io.Reader) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Create object directory structure: basePath/objectID[0:2]/objectID[2:4]/objectID
	dir1 := objectID[0:2]
	dir2 := objectID[2:4]
	objectDir := filepath.Join(n.BasePath, dir1, dir2)
	if err := os.MkdirAll(objectDir, 0755); err != nil {
		return fmt.Errorf("failed to create object directory: %w", err)
	}

	objectPath := filepath.Join(objectDir, objectID)
	file, err := os.Create(objectPath)
	if err != nil {
		return fmt.Errorf("failed to create object file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, data); err != nil {
		os.Remove(objectPath) // Clean up on error
		return fmt.Errorf("failed to write object data: %w", err)
	}

	return nil
}

// Retrieve reads object data from the storage node
func (n *Node) Retrieve(objectID string) (io.ReadCloser, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	dir1 := objectID[0:2]
	dir2 := objectID[2:4]
	objectPath := filepath.Join(n.BasePath, dir1, dir2, objectID)

	file, err := os.Open(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("object not found: %s", objectID)
		}
		return nil, fmt.Errorf("failed to open object file: %w", err)
	}

	return file, nil
}

// Exists checks if an object exists on this node
func (n *Node) Exists(objectID string) bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	dir1 := objectID[0:2]
	dir2 := objectID[2:4]
	objectPath := filepath.Join(n.BasePath, dir1, dir2, objectID)

	_, err := os.Stat(objectPath)
	return err == nil
}

// Delete removes an object from the storage node
func (n *Node) Delete(objectID string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	dir1 := objectID[0:2]
	dir2 := objectID[2:4]
	objectPath := filepath.Join(n.BasePath, dir1, dir2, objectID)

	if err := os.Remove(objectPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

// GetSize returns the size of an object in bytes
func (n *Node) GetSize(objectID string) (int64, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	dir1 := objectID[0:2]
	dir2 := objectID[2:4]
	objectPath := filepath.Join(n.BasePath, dir1, dir2, objectID)

	info, err := os.Stat(objectPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("object not found: %s", objectID)
		}
		return 0, fmt.Errorf("failed to stat object: %w", err)
	}

	return info.Size(), nil
}
