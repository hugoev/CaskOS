package hashring

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"sync"
)

// Node represents a storage node in the hash ring
type Node struct {
	ID       string
	Replicas int // Number of virtual nodes (replicas) for this physical node
}

// HashRing implements consistent hashing for node selection
type HashRing struct {
	mu            sync.RWMutex
	nodes         map[string]*Node
	sortedHashes  []uint32
	hashToNode    map[uint32]string
	virtualNodes  int // Number of virtual nodes per physical node
}

// NewHashRing creates a new hash ring with the specified number of virtual nodes per physical node
func NewHashRing(virtualNodes int) *HashRing {
	return &HashRing{
		nodes:        make(map[string]*Node),
		hashToNode:   make(map[uint32]string),
		virtualNodes: virtualNodes,
	}
}

// AddNode adds a physical node to the hash ring
func (hr *HashRing) AddNode(nodeID string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if _, exists := hr.nodes[nodeID]; exists {
		return // Node already exists
	}

	node := &Node{
		ID:       nodeID,
		Replicas: hr.virtualNodes,
	}
	hr.nodes[nodeID] = node

	// Add virtual nodes
	for i := 0; i < hr.virtualNodes; i++ {
		virtualKey := fmt.Sprintf("%s:%d", nodeID, i)
		hash := hr.hashKey(virtualKey)
		hr.hashToNode[hash] = nodeID
		hr.sortedHashes = append(hr.sortedHashes, hash)
	}

	// Sort hashes for binary search
	sort.Slice(hr.sortedHashes, func(i, j int) bool {
		return hr.sortedHashes[i] < hr.sortedHashes[j]
	})
}

// RemoveNode removes a physical node from the hash ring
func (hr *HashRing) RemoveNode(nodeID string) {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if _, exists := hr.nodes[nodeID]; !exists {
		return // Node doesn't exist
	}

	delete(hr.nodes, nodeID)

	// Remove virtual nodes
	newHashes := make([]uint32, 0, len(hr.sortedHashes))
	for _, hash := range hr.sortedHashes {
		if hr.hashToNode[hash] != nodeID {
			newHashes = append(newHashes, hash)
		} else {
			delete(hr.hashToNode, hash)
		}
	}
	hr.sortedHashes = newHashes
}

// GetNodes returns N nodes for a given key, ensuring they are distinct
func (hr *HashRing) GetNodes(key string, count int) []string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	if len(hr.nodes) == 0 {
		return []string{}
	}

	if count > len(hr.nodes) {
		count = len(hr.nodes)
	}

	hash := hr.hashKey(key)
	nodes := make([]string, 0, count)
	seen := make(map[string]bool)

	// Find the first node
	idx := hr.findNodeIndex(hash)
	if idx == -1 {
		return []string{}
	}

	// Collect distinct nodes
	for len(nodes) < count {
		nodeHash := hr.sortedHashes[idx]
		nodeID := hr.hashToNode[nodeHash]

		if !seen[nodeID] {
			nodes = append(nodes, nodeID)
			seen[nodeID] = true
		}

		idx = (idx + 1) % len(hr.sortedHashes)
		if idx == hr.findNodeIndex(hash) && len(nodes) < count {
			// We've wrapped around, break to avoid infinite loop
			break
		}
	}

	return nodes
}

// findNodeIndex finds the index of the first node with hash >= keyHash
func (hr *HashRing) findNodeIndex(keyHash uint32) int {
	idx := sort.Search(len(hr.sortedHashes), func(i int) bool {
		return hr.sortedHashes[i] >= keyHash
	})

	if idx == len(hr.sortedHashes) {
		// Wrap around to the first node
		return 0
	}

	return idx
}

// hashKey computes a 32-bit hash of a key
func (hr *HashRing) hashKey(key string) uint32 {
	h := sha256.Sum256([]byte(key))
	return uint32(h[0])<<24 | uint32(h[1])<<16 | uint32(h[2])<<8 | uint32(h[3])
}

// ListNodes returns all node IDs in the ring
func (hr *HashRing) ListNodes() []string {
	hr.mu.RLock()
	defer hr.mu.RUnlock()

	nodes := make([]string, 0, len(hr.nodes))
	for nodeID := range hr.nodes {
		nodes = append(nodes, nodeID)
	}
	return nodes
}

// NodeCount returns the number of physical nodes
func (hr *HashRing) NodeCount() int {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return len(hr.nodes)
}

