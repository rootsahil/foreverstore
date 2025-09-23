package main

//handles the encryption and decryption of our files

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"io"
)

// creates a new random 32 byte identifier
func generateID() string {
	buf := make([]byte, 32)
	io.ReadFull(rand.Reader, buf)
	return hex.EncodeToString(buf)
}

// takes a string and creates a short, fixed-sie fingerprint using MD5
func hashKey(key string) string {
	hash := md5.Sum([]byte(key))
	return hex.EncodeToString(hash[:])
}

// generates a new random 32 byte key suitable for AES-256 encryption
// key used to encrypt file
func newEncryptionKey() []byte {
	keyBuf := make([]byte, 32)
	io.ReadFull(rand.Reader, keyBuf)
	return keyBuf
}


// this transfers data from src to dst using our stream cipher
func copyStream(stream cipher.Stream, blockSize int, src io.Reader, dst io.Writer) (int, error) {
	
	//initialise buffer and set nw (number of bytes written) to blocksize (as before we would have written blockSize bytes before calling copy)
	var (
		buf = make([]byte, 32*1024)
		nw  = blockSize
	)

	for {
		//reads n bytes from source into buffer
		n, err := src.Read(buf)
		//if we have read something
		if n > 0 {
			// either encyrpt or decrypt n bytes in buffer in place
			stream.XORKeyStream(buf, buf[:n])
			// write these n bytes to the destination
			nn, err := dst.Write(buf[:n])
			if err != nil {
				return 0, err
			}
			// update number of bytes written
			nw += nn
		}
		// if theres no more in source, then break out of loop
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
	}
	return nw, nil
}

//src is our encrypted file, the very beginning contains IV, and the rest is scrambled data
func copyDecrypt(key []byte, src io.Reader, dst io.Writer) (int, error) {
	// block is an object that now understands how to encrypt and decrypt with our key
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}

	// read from src file and fill the iv bucket
	iv := make([]byte, block.BlockSize())
	if _, err := src.Read(iv); err != nil {
		return 0, err
	}

	// allows us to decrypt starting at iv
	stream := cipher.NewCTR(block, iv)
	// passes it to copy stream 
	return copyStream(stream, block.BlockSize(), src, dst)
}

func copyEncrypt(key []byte, src io.Reader, dst io.Writer) (int, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return 0, err
	}

	iv := make([]byte, block.BlockSize()) // 16 bytes

	// rand.Reader gives unpredictable data, and we read 16 bytes into iv
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return 0, err
	}

	// prepend the IV to the file.
	if _, err := dst.Write(iv); err != nil {
		return 0, err
	}

	stream := cipher.NewCTR(block, iv)
	return copyStream(stream, block.BlockSize(), src, dst)
}