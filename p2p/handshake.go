package p2p

//Any function that takes in a perr and returns an error is of type Handshake func
type HandshakeFunc func(Peer) error

//Default handshake where you just want to send quickly
func NOPHandshakeFunc(Peer) error { return nil }