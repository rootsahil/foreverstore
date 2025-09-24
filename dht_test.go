package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	//"fmt"
	"net"
	"testing"
)

type mockPinger struct {
	shouldRespond bool
}

func (mp *mockPinger) Ping(node Node) bool {
	return mp.shouldRespond
}

// newNodeForTest 
func newNodeForTest(id string) Node {
	hash := sha1.Sum([]byte(id))
	return Node{
		ID:   hash[:],
		IP:   net.ParseIP("127.0.0.1"),
		Port: 8080,
	}
}

// Helper to create a new random ID for testing.
func newRandomID() []byte {
	id := make([]byte, 20) // 20 bytes for SHA1
	rand.Read(id)
	return id
}

func TestRoutingTable_Add(t *testing.T) {
	selfID := newRandomID()
	rt := NewRoutingTable(selfID)
	pinger := &mockPinger{shouldRespond: true}

	// === Test Case 1: Add a new node ===
	newNode := Node{ID: newRandomID()}
	rt.Add(pinger, newNode)
	bucketIndex := rt.getBucketIndex(newNode.ID)
	if rt.buckets[bucketIndex].Len() != 1 {
		t.Fatalf("Case 1 Failed: expected 1 node, got %d", rt.buckets[bucketIndex].Len())
	}

	// === Test Case 2: Add existing node, should move to front ===
	anotherNode := Node{ID: newRandomID()}
	rt.Add(pinger, anotherNode) // Add a second node
	rt.Add(pinger, newNode)     // Re-add the first node
	
    // We need to find which bucket 'anotherNode' went into, which might be different
    anotherNodeBucketIndex := rt.getBucketIndex(anotherNode.ID)
    if anotherNodeBucketIndex == bucketIndex {
        if rt.buckets[bucketIndex].Len() != 2 {
            t.Fatalf("Case 2 Failed: expected 2 nodes in bucket, got %d", rt.buckets[bucketIndex].Len())
        }
    }
	if !bytes.Equal(rt.buckets[bucketIndex].Front().Value.(Node).ID, newNode.ID) {
		t.Error("Case 2 Failed: existing node was not moved to the front")
	}

	// === Test Case 3 & 4: Test a full bucket ===
	// To reliably test a full bucket, let's pick one and fill it.
	targetBucketIndex := rt.getBucketIndex(newRandomID())
	
	// Create k nodes that are guaranteed to be in the target bucket
	nodesInBucket := make([]Node, k)
	for i := 0; i < k; i++ {
		var node Node
		// Keep creating random nodes until we get one for the target bucket
		for {
			node = Node{ID: newRandomID()}
			if rt.getBucketIndex(node.ID) == targetBucketIndex {
				nodesInBucket[i] = node
				rt.Add(pinger, node)
				break
			}
		}
	}
	if rt.buckets[targetBucketIndex].Len() != k {
		t.Fatalf("Case 3 Failed: Bucket should be full with %d nodes, but has %d", k, rt.buckets[targetBucketIndex].Len())
	}
	
	//oldestNode := rt.buckets[targetBucketIndex].Back().Value.(Node)

	// Case 3: Add to full bucket, oldest is UNRESPONSIVE
	pinger.shouldRespond = false
	overflowNode := Node{ID: newRandomID()}
    // Force the overflow node into the target bucket
    for rt.getBucketIndex(overflowNode.ID) != targetBucketIndex {
        overflowNode.ID = newRandomID()
    }
	rt.Add(pinger, overflowNode)

	if rt.buckets[targetBucketIndex].Len() != k {
		t.Error("Case 3 Failed: Bucket size should remain k")
	}
	if !bytes.Equal(rt.buckets[targetBucketIndex].Front().Value.(Node).ID, overflowNode.ID) {
		t.Error("Case 3 Failed: New node should be at the front")
	}

	// Case 4: Add to full bucket, oldest is RESPONSIVE
	pinger.shouldRespond = true
	anotherOverflowNode := Node{ID: newRandomID()}
    // Force this node into the target bucket as well
    for rt.getBucketIndex(anotherOverflowNode.ID) != targetBucketIndex {
        anotherOverflowNode.ID = newRandomID()
    }
	rt.Add(pinger, anotherOverflowNode)
	
	if rt.buckets[targetBucketIndex].Len() != k {
		t.Error("Case 4 Failed: Bucket size should remain k")
	}
	if bytes.Equal(rt.buckets[targetBucketIndex].Front().Value.(Node).ID, anotherOverflowNode.ID) {
		t.Error("Case 4 Failed: New node should have been discarded")
	}
}