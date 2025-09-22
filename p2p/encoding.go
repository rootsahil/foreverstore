package p2p

import (
	"encoding/gob"
	"io"
)

//defines how we receive / interpret messages fom the network 

type Decoder interface {
	Decode(io.Reader, *RPC) error
}

type GOBDecoder struct{}

//gob creates a new object that can read encoded data from r(TCP stream)
//it then tries to read bytes from the stream and fill in the msg struct
func (dec GOBDecoder) Decode(r io.Reader, msg *RPC) error {
	return gob.NewDecoder(r).Decode(msg)
}

type DefaultDecoder struct{}

func (dec DefaultDecoder) Decode(r io.Reader, msg *RPC) error {
	peekBuf := make([]byte, 1)
	if _, err := r.Read(peekBuf); err != nil {
		return nil
	}

	// here we determine if it is a stream of message, if so we set the RPC attribute
	stream := peekBuf[0] == IncomingStream
	if stream {
		msg.Stream = true
		return nil
	}

	// n is number of bytes read
	buf := make([]byte, 1028)
	n, err := r.Read(buf)
	if err != nil {
		return err
	}

	msg.Payload = buf[:n]

	return nil
}