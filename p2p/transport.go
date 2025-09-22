package p2p

// Peer represents the remote node
type Peer interface{}

// Transport is anything that handles communication between nodes
type Transport interface{

	ListenAndAccept() error

}
