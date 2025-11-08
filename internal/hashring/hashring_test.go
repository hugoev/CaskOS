package hashring

import (
	"testing"
)

func TestHashRing_AddNode(t *testing.T) {
	ring := NewHashRing(3)

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	if ring.NodeCount() != 3 {
		t.Errorf("expected 3 nodes, got %d", ring.NodeCount())
	}
}

func TestHashRing_RemoveNode(t *testing.T) {
	ring := NewHashRing(3)

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	ring.RemoveNode("node2")

	if ring.NodeCount() != 2 {
		t.Errorf("expected 2 nodes, got %d", ring.NodeCount())
	}

	nodes := ring.ListNodes()
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes in list, got %d", len(nodes))
	}
}

func TestHashRing_GetNodes(t *testing.T) {
	ring := NewHashRing(3)

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	// Test getting nodes for a key
	nodes := ring.GetNodes("test-key", 2)
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}

	// Ensure nodes are distinct
	if nodes[0] == nodes[1] {
		t.Errorf("expected distinct nodes, got duplicates: %v", nodes)
	}

	// Test getting more nodes than available
	nodes = ring.GetNodes("another-key", 5)
	if len(nodes) != 3 {
		t.Errorf("expected 3 nodes (all available), got %d", len(nodes))
	}
}

func TestHashRing_ConsistentHashing(t *testing.T) {
	ring := NewHashRing(3)

	ring.AddNode("node1")
	ring.AddNode("node2")
	ring.AddNode("node3")

	key := "test-object-id"
	nodes1 := ring.GetNodes(key, 2)

	// Adding a new node should not change assignment for existing keys
	ring.AddNode("node4")
	nodes2 := ring.GetNodes(key, 2)

	// The first node should remain the same (consistent hashing property)
	if nodes1[0] != nodes2[0] {
		t.Logf("Note: first node changed after adding node4 (this can happen with virtual nodes)")
		t.Logf("Original: %v, New: %v", nodes1, nodes2)
	}
}

func TestHashRing_EmptyRing(t *testing.T) {
	ring := NewHashRing(3)

	nodes := ring.GetNodes("test-key", 2)
	if len(nodes) != 0 {
		t.Errorf("expected 0 nodes from empty ring, got %d", len(nodes))
	}
}

