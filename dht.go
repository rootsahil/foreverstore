package main

import (
	"container/list" // We'll use this for the k-buckets
	"net"
	"crypto/sha1"
	"bytes"
)

type Pinger interface {
	Ping(node Node) bool
}

// The size of the node ID in bits. SHA-1 produces a 160-bit hash.
const idLength = 160

// The 'k' parameter in Kademlia. It's the maximum number of nodes
// that can be in a single k-bucket. 
const k = 20

// Node represents a single peer in the DHT network.
// This is like a contact card.
type Node struct {
	ID   []byte        // Nodes unique 160-bit Kademilia ID
	IP   net.IP        // Nodes IP address
	Port int           // Nodes port number (which port its listening on)
}

// RoutingTable is the "smart contact list" for a node.
// It contains k-buckets to organize known peers by distance.
type RoutingTable struct {
	// The ID of the node that owns this routing table.
	selfID []byte
	// The k-buckets. There is one bucket (doubly linked list) for each bit of the ID.
	buckets [idLength]*list.List 
}

// NewRoutingTable creates and initializes a new RoutingTable.
func NewRoutingTable(selfID []byte) *RoutingTable {
	rt := &RoutingTable{
		selfID: selfID,
	}
	// Initialize all the k-bucket lists.
	for i := 0; i < idLength; i++ {
		rt.buckets[i] = list.New()
	}
	return rt
}

// Add adds a new node to the routing table.
func (rt *RoutingTable) Add(pinger Pinger, node Node) {
	// A node should not add itself to its own routing table.
	if bytes.Equal(rt.selfID, node.ID) {
		return
	}

	bucketIndex := rt.getBucketIndex(node.ID)
	bucket := rt.buckets[bucketIndex]

	// Case 1: The node already exists.
	// We move it to the front of the list to mark it as "recently seen".
	for e := bucket.Front(); e != nil; e = e.Next() {
		if bytes.Equal(e.Value.(Node).ID, node.ID) {
			bucket.MoveToFront(e)
			return
		}
	}

	// Case 2: The bucket is full.
	if bucket.Len() >= k {
		// Ping the least recently seen node (the one at the back).
		oldestNode := bucket.Back().Value.(Node)
		if pinger.Ping(oldestNode) {
			// If it responds, move it to the front and discard the new node.
			bucket.MoveToFront(bucket.Back())
			return
		} else {
			// If it doesn't respond, remove it from the back.
			bucket.Remove(bucket.Back())
		}
	}

	// Case 3: The bucket has space, and the node is new.
	// Add the new node to the front of the bucket's list.
	bucket.PushFront(node)
}

// getBucketIndex calculates the correct k-bucket index for a given node ID.
func (rt *RoutingTable) getBucketIndex(otherID []byte) int {
	// Calculate the XOR distance.
	distance := xor(rt.selfID, otherID)

	// Count the number of leading zeros to find the bucket index.
	// This is the core of the Kademlia distance metric.
	var leadingZeros int
	for _, b := range distance {

		// bytes worth of data is same
		if b == 0 {
			leadingZeros += 8
		// byte slice not same
		} else {
			// Find the first set bit in the byte.
			for j := 7; j >= 0; j-- {
				if (b>>j)&1 != 0 {
					leadingZeros += (7 - j)
					return idLength - 1 - leadingZeros
				}
			}
		}
	}

	return idLength - 1 - leadingZeros
}

// xor calculates the bitwise XOR distance between two node IDs.
func xor(a, b []byte) []byte {         // []bytes means a and b are list of bytes
	result := make([]byte, len(a))
	for i := 0; i < len(a); i++ {
		result[i] = a[i] ^ b[i]
	}
	return result
}

// Helper function to create a new random ID for testing.
func newID() []byte {
	hash := sha1.Sum(nil) // Just an example, in reality you'd use a real random source
	return hash[:]
}