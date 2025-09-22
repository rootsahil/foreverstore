package p2p

import "net"

// Peer represents the remote node
type Peer interface{

	net.Conn
	Send([]byte) error
	CloseStream()

}

// Transport is anything that handles communication between nodes
type Transport interface{

	Addr() string
	Dial(string) error
	ListenAndAccept() error
	Consume() <- chan RPC
	Close() error 

}
