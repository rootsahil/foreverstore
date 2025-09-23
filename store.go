package main

import (
	//"crypto/sha1"
	//"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

const defaultRootFolderName = "ggnetwork"

// PathName: "f49ca/b28b3/b1385"
// FileName: "f49cab28b3b13850b6a7"

type PathKey struct {
	PathName string
	Filename string
}

// gets the very first folder in the nested directory
// useful for Delete function
// returns f49ca
func (p PathKey) FirstPathName() string {
	paths := strings.Split(p.PathName, "/")
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

// takes pathname and filename and joins them together with a / in the middle
// "f49ca/b28b3/b1385/f49cab28b3b13850b6a7"
func (p PathKey) FullPath() string {
	return fmt.Sprintf("%s/%s", p.PathName, p.Filename)
}

type PathTransformFunc func(string) PathKey

// takes in a key (the hash of the chunk)
// 
func CASPathTransformFunc(key string) PathKey {
	//hash := sha1.Sum([]byte(key))
	//hashStr := hex.EncodeToString(hash[:])

	hashStr := key

	blocksize := 5
	sliceLen := len(hashStr) / blocksize
	paths := make([]string, sliceLen)

	for i := 0; i < sliceLen; i++ {
		from, to := i*blocksize, (i*blocksize)+blocksize
		paths[i] = hashStr[from:to]
	}

	return PathKey{
		PathName: strings.Join(paths, "/"),
		Filename: hashStr,
	}
}

var DefaultPathTransformFunc = func(key string) PathKey {
	return PathKey{
		PathName: key,
		Filename: key,
	}
}


// how we configure the store
// root: our main folder for storage
// pathtransformfun: special rule for how we organise files
type StoreOpts struct {
	Root              string
	PathTransformFunc PathTransformFunc
}

type Store struct {
	StoreOpts
}

func NewStore(opts StoreOpts) *Store {
	if opts.PathTransformFunc == nil {
		opts.PathTransformFunc = DefaultPathTransformFunc
	}
	if len(opts.Root) == 0 {
		opts.Root = defaultRootFolderName
	}

	return &Store{
		StoreOpts: opts,
	}
}


// asks OS if we have a file in "ggnetwork/user_alice/f49ca/b28b3/.../f49ca..."
func (s *Store) Has(id string, key string) bool {
	// generates f49ca/b28b3/.../f49ca...
	pathKey := s.PathTransformFunc(key)
	//combines ggnetwork | user_alice | and f49ca/b28b3/.../f49ca...
	fullPathWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FullPath())
	//asks OS if a file exists in this location
	_, err := os.Stat(fullPathWithRoot)
	return !errors.Is(err, os.ErrNotExist)
}

func (s *Store) Clear() error {
	return os.RemoveAll(s.Root)
}

// deletes a file chunk
// creates the string "ggnetwork/user_alice/f49ca"
// deletes everything with f49ca including f49ca
func (s *Store) Delete(id string, key string) error {
	pathKey := s.PathTransformFunc(key)
    
    // Construct the FULL path to the specific file.
	fullPathToFile := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FullPath())

	log.Printf("deleted [%s] from disk", pathKey.Filename)

    // Use os.Remove(), which deletes a single file, not a whole directory.
	return os.Remove(fullPathToFile)
}

// func (s *Store) Delete(id string, key string) error {
// 	pathKey := s.PathTransformFunc(key)

// 	defer func() {
// 		log.Printf("deleted [%s] from disk", pathKey.Filename)
// 	}()

// 	firstPathNameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FirstPathName())

// 	return os.RemoveAll(firstPathNameWithRoot)
// }

// public facing - allows us sto save data to a file
func (s *Store) Write(id string, key string, r io.Reader) (int64, error) {
	return s.writeStream(id, key, r)
}

func (s *Store) openFileForWriting(id string, key string) (*os.File, error) {
	pathKey := s.PathTransformFunc(key)
	pathNameWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.PathName)
	//creates directory if it doesnt exist 
	if err := os.MkdirAll(pathNameWithRoot, os.ModePerm); err != nil {
		return nil, err
	}
	// now the folders exists, it consutrcts the full path including file name
	fullPathWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FullPath())
	// create a new blank file at this exact location, if a file exists wipe it
	return os.Create(fullPathWithRoot)
}

func (s *Store) writeStream(id string, key string, r io.Reader) (int64, error) {
	f, err := s.openFileForWriting(id, key)
	if err != nil {
		return 0, err
	}
	// reads all the data from source r and writes it directly to f 
	return io.Copy(f, r)
}

func (s *Store) WriteDecrypt(encKey []byte, id string, key string, r io.Reader) (int64, error) {
	f, err := s.openFileForWriting(id, key)
	if err != nil {
		return 0, err
	}
	n, err := copyDecrypt(encKey, r, f)
	return int64(n), err
}

func (s *Store) Read(id string, key string) (int64, io.Reader, error) {
	return s.readStream(id, key)
}

func (s *Store) readStream(id string, key string) (int64, io.ReadCloser, error) {
	pathKey := s.PathTransformFunc(key)
	fullPathWithRoot := fmt.Sprintf("%s/%s/%s", s.Root, id, pathKey.FullPath())

	file, err := os.Open(fullPathWithRoot)
	if err != nil {
		return 0, nil, err
	}
	// asks the OS for metadata about the file it just opened
	fi, err := file.Stat()
	if err != nil {
		return 0, nil, err
	}
	// file is the open file handle (io.Reader) which you can now use to stream the file's content
	return fi.Size(), file, nil
}