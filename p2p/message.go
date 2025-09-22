package p2p

// incoming message are small and sent all at once e.g. Hello / Ping
// stream is long lived and sent in multiple pieces
// so when the first byte is 0x2 the system knows:
// hey don't try to decoe the rest of the message wait for it to finish. 
const (
	IncomingMessage = 0x1
	IncomingStream = 0x2
)


// RPC goes into a channel where someone can say, 
// oh i got a messageg from 192.168.1.5 that say's hello, let's do something.
type RPC struct {
	From string
	Payload []byte
	Stream bool
}