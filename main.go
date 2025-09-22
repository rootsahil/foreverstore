package main

import (
    "log"
    "fmt"

    "github.com/rootsahil/foreverstore/p2p"
)

func OnPeer(p2p.Peer) error {
    fmt.Println("function that runs after connection")
    return nil
}

func main() {

    opts := p2p.TCPTransportOpts {
        ListenAddr: ":3000",
        HandshakeFunc: p2p.NOPHandshakeFunc,
        Decoder: p2p.DefaultDecoder{},
        OnPeer: OnPeer,
    }
    tr := p2p.NewTCPTransport(opts)

    go func() {
        for {
            msg := <- tr.Consume()
            fmt.Printf("%+v\n", msg)
        }
    }()

    if err := tr.ListenAndAccept(); err != nil {
        log.Fatal(err)
    }

    select {}
}
