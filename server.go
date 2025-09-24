package main

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/rootsahil/foreverstore/p2p"
)

// config for how our server will behave
type FileServerOpts struct {
	ID                string // used to identify itself on the network
	EncKey            []byte // used to encrypt / decrypt files
	StorageRoot       string // which sector of the HDD are we storing things
	PathTransformFunc PathTransformFunc // how do we organise our sector
	Transport         p2p.Transport // server uses this to communicate with others on the server
	BootstrapNodes    []string // when server starts up, it uses this list as its first point of contact to connect to the network
}

type FileServer struct {
	FileServerOpts

	peerLock sync.Mutex // when a peer connects / disconencts it must first grab this lock
	peers    map[string]p2p.Peer // the servers contact list, keeps track of all other connected peers

	store  *Store // server will use this to manage files on its local HDD
	quitch chan struct{}
}

func NewFileServer(opts FileServerOpts) *FileServer {
	storeOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}

	if len(opts.ID) == 0 {
		opts.ID = generateID()
	}

	return &FileServer{
		FileServerOpts: opts,
		store:          NewStore(storeOpts),
		quitch:         make(chan struct{}),
		peers:          make(map[string]p2p.Peer),
	}
}

// broadcast a single message to every single peer we are connected to
func (s *FileServer) broadcast(msg *Message) error {

	// 1) preparing the message for travel 
	buf := new(bytes.Buffer)
	if err := gob.NewEncoder(buf).Encode(msg); err != nil {
		return err
	}
	// buf now contains our serialised message

	// 2) sending the package to everyone
	for _, peer := range s.peers {
		peer.Send([]byte{p2p.IncomingMessage}) // sending a heads up that we are sending a short message
		if err := peer.Send(buf.Bytes()); err != nil {
			return err
		}
	}

	return nil
}

// generic message
type Message struct {
	Payload any
}

// server broadcasts that a new file has been stored
type MessageStoreFile struct {
	ID   string
	Key  string
	Size int64
}

// server broadcasts a request for a file it does not have locally
type MessageGetFile struct {
	ID  string
	Key string
}

// server is trying to download a file (whether its stored locally or on a peer's device)
func (s *FileServer) Get(key string) (io.Reader, error) {
	// First, check if we have the file locally
	if s.store.Has(s.ID, key) {
		fmt.Printf("[%s] serving file (%s) from local disk\n", s.Transport.Addr(), key)
		_, r, err := s.store.Read(s.ID, key)
		return r, err
	}

	fmt.Printf("[%s] dont have file (%s) locally, fetching from network...\n", s.Transport.Addr(), key)

	// Broadcast the "get file" request
	msg := Message{
		Payload: MessageGetFile{
			ID:  s.ID,
			Key: hashKey(key), // Note: A better key would be the actual content hash.
		},
	}
	if err := s.broadcast(&msg); err != nil {
		return nil, err
	}

	// NEW: Create a channel to wait for the first successful file transfer.
	// This channel will hold the io.Reader for the retrieved file.
	ch := make(chan io.Reader)
	errCh := make(chan error) // A channel for any errors that occur.
	
	//Loop through the peers and start a goroutine for each one.
	for _, peer := range s.peers {
		go func(p p2p.Peer) {
			// First, read the file size from the peer.
			var fileSize int64
			err := binary.Read(p, binary.LittleEndian, &fileSize)
			if err != nil {
				// If we can't read the size, this peer isn't sending the file.
				// We can just log it and the goroutine will exit.
				// fmt.Printf("Error reading file size from %s: %v\n", p.RemoteAddr(), err)
				return
			}

			// Decrypt the incoming stream and write it to our local store.
			_, err = s.store.WriteDecrypt(s.EncKey, s.ID, key, io.LimitReader(p, fileSize))
			if err != nil {
				errCh <- fmt.Errorf("error writing decrypted file from peer %s: %w", p.RemoteAddr(), err)
				return
			}
			
			p.CloseStream()
			fmt.Printf("[%s] received file (%s) over the network from (%s)\n", s.Transport.Addr(), key, p.RemoteAddr())

			// Success! Read the file back from our store to get an io.Reader.
			_, r, err := s.store.Read(s.ID, key)
			if err != nil {
				errCh <- fmt.Errorf("error reading newly saved file: %w", err)
				return
			}
			
			// Send the reader into the success channel.
			ch <- r
		}(peer)
	}

	// Wait for either the first successful download or a timeout.
	select {
	case r := <-ch:
		return r, nil
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Second): // Added a 5-second timeout.
		return nil, fmt.Errorf("timeout waiting for file from the network")
	}
}

// the servers upload and share logic 
// the server safes a file to its own disk and then sends an encrypted copy to all its connected peers
func (s *FileServer) Store(key string, r io.Reader) error {
	var (
		fileBuffer = new(bytes.Buffer)
		tee        = io.TeeReader(r, fileBuffer)
	)

	// the server writes to the store, as it writes it pulls data from the reader (tee) who gives it the data, and tee writes the same data to buffer
	size, err := s.store.Write(s.ID, key, tee)
	if err != nil {
		return err
	}

	msg := Message{
		Payload: MessageStoreFile{
			ID:   s.ID,
			Key:  hashKey(key),
			Size: size + 16,
		},
	}

	if err := s.broadcast(&msg); err != nil {
		return err
	}

	time.Sleep(time.Millisecond * 5)

	// create a list of peers
	peers := []io.Writer{}
	for _, peer := range s.peers {
		peers = append(peers, peer)
	}
	mw := io.MultiWriter(peers...)
	mw.Write([]byte{p2p.IncomingStream})
	n, err := copyEncrypt(s.EncKey, fileBuffer, mw)
	if err != nil {
		return err
	}

	fmt.Printf("[%s] received and written (%d) bytes to disk\n", s.Transport.Addr(), n)

	return nil
}

// a clean way to shut down our running server
func (s *FileServer) Stop() {
	close(s.quitch)
}

//safely adds a newly connected peer to our server's internal contact list
func (s *FileServer) OnPeer(p p2p.Peer) error {
	// obtain lock
	s.peerLock.Lock()
	// release lock
	defer s.peerLock.Unlock()

	s.peers[p.RemoteAddr().String()] = p

	log.Printf("connected with remote %s", p.RemoteAddr())

	return nil
}

func (s *FileServer) loop() {

	// when server ends, we must close off our transportation system (locking our entrance/exit)
	defer func() {
		log.Println("file server stopped due to error or user quit action")
		s.Transport.Close()
	}()

	// infinite loop
	for {
		select {
		// server waits for any mail to arrive
		case rpc := <-s.Transport.Consume():
			var msg Message
			// create a reader to understand rpc payload, and decode the message into msg
			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				log.Println("decoding error: ", err)
			}
			// handle the decoded message and who its from
			if err := s.handleMessage(rpc.From, &msg); err != nil {
				log.Println("handle message error: ", err)
			}
		
		// elsewhere in code, somewhere has closed s.quitch so we clsoe server
		case <-s.quitch:
			return
		}
	}
}


// main entry point for all incoming messagesm
// passes the from, and payload to the correct function depending on whether its a store or get message
func (s *FileServer) handleMessage(from string, msg *Message) error {
	switch v := msg.Payload.(type) {
	case MessageStoreFile:
		return s.handleMessageStoreFile(from, v)
	case MessageGetFile:
		return s.handleMessageGetFile(from, v)
	}

	return nil
}

func (s *FileServer) handleMessageGetFile(from string, msg MessageGetFile) error {
	
	// checks if we have the message 
	if !s.store.Has(msg.ID, msg.Key) {
		return fmt.Errorf("[%s] need to serve file (%s) but it does not exist on disk", s.Transport.Addr(), msg.Key)
	}

	fmt.Printf("[%s] serving file (%s) over the network\n", s.Transport.Addr(), msg.Key)

	// reads the file from store, to get file size and readable stream (r)
	fileSize, r, err := s.store.Read(msg.ID, msg.Key)
	if err != nil {
		return err
	}

	if rc, ok := r.(io.ReadCloser); ok {
		fmt.Println("closing readCloser")
		defer rc.Close()
	}

	// looks up the requesting peer in its contact list to get the correct connection to send back to
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer %s not in map", from)
	}

	// First send the "incomingStream" byte to the peer and then we can send
	// the file size as an int64.
	peer.Send([]byte{p2p.IncomingStream}) // headsup 
	binary.Write(peer, binary.LittleEndian, fileSize) // sends file size so receiver knows how much data to expect
	n, err := io.Copy(peer, r) // uses io.Copy to stream the entire files content over the network to the peer
	if err != nil {
		return err
	}

	fmt.Printf("[%s] written (%d) bytes over the network to %s\n", s.Transport.Addr(), n, from)

	return nil
}

func (s *FileServer) handleMessageStoreFile(from string, msg MessageStoreFile) error {
	
	// looksup peer in contact list
	peer, ok := s.peers[from]
	if !ok {
		return fmt.Errorf("peer (%s) could not be found in the peer list", from)
	}

	// io.LimitReader(peer, msg.size) is the incoming stream of data, and we write it to the path id/key etc
	n, err := s.store.Write(msg.ID, msg.Key, io.LimitReader(peer, msg.Size))
	if err != nil {
		return err
	}

	fmt.Printf("[%s] written %d bytes to disk\n", s.Transport.Addr(), n)

	// once file is completly written, we signal to peer that the transfer is complete which unpauses the sending peer's conneciton
	peer.CloseStream()

	return nil
}

// server's way of joining to the existing peer-to-peer network 
func (s *FileServer) bootstrapNetwork() error {
	// s.BootStrapNodes is a pre configured list of address for a few stable peers
	for _, addr := range s.BootstrapNodes {
		if len(addr) == 0 {
			continue
		}

		// for each address is starts a new goroutine to attempt to connect to them
		go func(addr string) {
			fmt.Printf("[%s] attemping to connect with remote %s\n", s.Transport.Addr(), addr)
			// dials to the address - if its a success, both server and remote server trigger onpeer func to add each other to s.peer
			if err := s.Transport.Dial(addr); err != nil {
				log.Println("dial error: ", err)
			}
		}(addr)
	}

	return nil
}

func (s *FileServer) Start() error {
	fmt.Printf("[%s] starting fileserver...\n", s.Transport.Addr())

	// tells underlying P2P network to open a port and start listening for connections
	if err := s.Transport.ListenAndAccept(); err != nil {
		return err
	}

	// we are now open, so we reach out to connect with known peers form the list
	s.bootstrapNetwork()

	// it has started listening, and connected to the P2P network, and so we run the loop
	s.loop()

	return nil
}

// automatically ran before the main program starts
// gob needs to know a bout the specific cucstom types we plan to send over the network 
func init() {
	gob.Register(MessageStoreFile{})
	gob.Register(MessageGetFile{})
}