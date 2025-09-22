package p2p

import (
	"net"
	"sync"
)

// Peer represents the remote node
type Peer interface{}

// Transport is anything that handles communication between nodes
type Transport interface{}

type TCPTransport struct {
	listenAddress string
	listener      net.Listener

	mu    sync.RWMutex
	peers map[net.Addr]Peer
}

// Constructor for the transport
func NewTCPTransport(listenAddr string) *TCPTransport {
	return &TCPTransport{
		listenAddress: listenAddr,
		peers:         make(map[net.Addr]Peer),
	}
}
