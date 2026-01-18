package key_store

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

// ////
// metadata for a file reference stored in memory
// this is the metadata for the file itself as a whole
// //
type FileMetaData struct {
	FileHash    [HashSize]byte   `toml:"file_hash"`
	TotalSize   uint64           `toml:"total_size"`
	Modified    int64            `toml:"modified"`
	Replicate   uint32           `toml:"replicate"`
	Permissions uint32           `toml:"permissions"`
	Signature   [CryptoSize]byte `toml:"signature"`
	TTL         uint64           `toml:"ttl"`
	ChunkSize   uint32           `toml:"chunk_size"`
	TotalChunks uint32           `toml:"total_chunks"`
}

// the top level reference to a file in memory
// includes the metadata for reconstructing the file
// as well as references to its KNOWN chunks
type FileReference struct {
	MetaData FileMetaData `toml:"metadata"`
	Chunks   []*FileChunk `toml:"references,omitempty"`
}

// this is a reference to locally stored chunks of a file
// includes a pointer to the top level files metadata for reconstruction
// each file reference maps a chunk of the data to its storage location on disk
type FileChunk struct {
	Key       [KeySize]byte `toml:"key"`
	Size      uint32        `toml:"chunk_size"`
	FileIndex uint32        `toml:"chunk_index"`
	// location can vary in type by protocol?
	// 	file = path/to/file
	// 	net = server address
	Location string         `toml:"location"`
	Protocol string         `toml:"protocol"`
	DataHash [HashSize]byte `toml:"data_hash"`
}

func NewFileChunk(data []byte, protocol string, location string) *FileChunk {
	// 	calculate chunk's dht routing id
	// 	using fileHash | FileIndex allows for reference level modifications
	// 	see updatekey()
	// 	for data integrity, likely want to use sha1.sum(chunk_data)

	// default to content based addressing
	key := sha1.Sum(data)
	f := &FileChunk{
		Key:       key,
		Location:  location,
		Size:      uint32(len(data)),
		FileIndex: 0,
		Protocol:  protocol,
		DataHash:  sha256.Sum256(data),
	}

	return f
}

func (fc *FileChunk) updateKey(
	fileHash [HashSize]byte,
	fileIndex uint32,
) {
	// deterministic: SHA-1 of (fileHash || chunkIndex)
	buf := make([]byte, len(fileHash)+8)
	copy(buf, fileHash[:])
	binary.LittleEndian.PutUint64(buf[len(fileHash):], uint64(fileIndex))
	fc.Key = sha1.Sum(buf)
	fc.FileIndex = fileIndex
}

type KeyStore struct {
	references map[[HashSize]byte]*FileReference
	registry   map[[KeySize]byte]*FileChunk

	storage  string
	metadata string

	lock sync.RWMutex
}

func BootstrapKeyStore(storageDir string) (*KeyStore, error) {
	ks := &KeyStore{
		registry:   make(map[[KeySize]byte]*FileChunk),
		references: make(map[[HashSize]byte]*FileReference),
		storage:    storageDir,
		metadata:   "default",
	}

	ks.bootstrapDirs()

	entries, err := os.ReadDir(ks.metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".toml") {
			// extract hash from filename
			hashStr := strings.TrimSuffix(entry.Name(), ".toml")
			var fileHash [HashSize]byte
			hashBytes, err := hex.DecodeString(hashStr)
			if err != nil {
				fmt.Printf("Warning: invalid metadata filename %s: %v\n", entry.Name(), err)
				continue
			}
			copy(fileHash[:], hashBytes)

			// load complete file struct
			// maps the local files in metadata/ to its chunks in storage/**.dht
			var file FileReference
			if _, err := toml.DecodeFile(filepath.Join(ks.metadata, entry.Name()), &file); err != nil {
				fmt.Printf("Warning: failed to decode file %s: %v\n", entry.Name(), err)
				continue
			}

			// store file in memory
			ks.references[fileHash] = &file

			// also store chunk references
			for _, ref := range file.Chunks {
				if ref != nil {
					ks.registry[ref.Key] = ref
				}
			}
		}
	}

	// verify chunks and handle orphaned metadata
	if err := ks.identifyOrphans(); err != nil {
		fmt.Printf("Warning: error during chunk verification: %v\n", err)
	}

	return ks, nil
}

// return copies in slice of all file references
func (ks *KeyStore) ListLocalChunks() []*FileChunk {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	chunks := make([]*FileChunk, 0, len(ks.registry))
	for _, chunk := range ks.registry {
		chunks = append(chunks, chunk) // copy by value
	}
	return chunks
}

// return copies in slice
func (ks *KeyStore) ListKnownReferences() []*FileReference {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	entries := make([]*FileReference, 0, len(ks.references))
	for _, file := range ks.references {
		entries = append(entries, file) // copy the metadata from the file struct
	}
	return entries
}

func (ks *KeyStore) Cleanup() error {
	ks.lock.Lock()
	defer ks.lock.Unlock()

	// clean up chunk files
	for id, chunk := range ks.registry {
		if err := os.Remove(chunk.Location); err != nil {
			return fmt.Errorf("failed to delete chunk %x: %w", id, err)
		}
	}

	// clean up metadata files
	metadataDir := filepath.Join(ks.storage, "metadata")
	if err := os.RemoveAll(metadataDir); err != nil {
		return fmt.Errorf("failed to delete metadata directory: %w", err)
	}

	// reset the maps
	ks.registry = make(map[[KeySize]byte]*FileChunk)
	ks.references = make(map[[HashSize]byte]*FileReference) // changed to *file
	return nil
}

func (ks *KeyStore) CleanupKDHT() error {
	err := ks.CleanupExtensions(".kdht")
	return err
}

func (ks *KeyStore) CleanupMetaData() error {
	err := ks.CleanupExtensions(".toml")
	return err
}

func (ks *KeyStore) CleanupExtensions(extensions ...string) error {
	ks.lock.Lock()
	defer ks.lock.Unlock()

	// create a map for quick extension lookup
	validExt := make(map[string]bool)
	for _, ext := range extensions {
		validExt[ext] = true
	}

	// clean up chunk files
	for id, chunk := range ks.registry {
		if validExt[filepath.Ext(chunk.Location)] {
			if err := os.Remove(chunk.Location); err != nil {
				return fmt.Errorf("failed to delete chunk %x: %w", id, err)
			}
		}
	}

	// clean up metadata directory
	metadataDir := filepath.Join(ks.storage, "metadata")
	if entries, err := os.ReadDir(metadataDir); err == nil {
		for _, entry := range entries {
			if validExt[filepath.Ext(entry.Name())] {
				fullPath := filepath.Join(metadataDir, entry.Name())
				if err := os.Remove(fullPath); err != nil {
					return fmt.Errorf("failed to delete metadata file %s: %w", entry.Name(), err)
				}
			}
		}
	}

	// reset the maps
	ks.registry = make(map[[KeySize]byte]*FileChunk)
	ks.references = make(map[[HashSize]byte]*FileReference)
	return nil
}

func (ks *KeyStore) identifyOrphans() error {
	ks.lock.Lock()
	defer ks.lock.Unlock()

	metadataDir := ks.metadata
	orphansRoot := filepath.Join(ks.storage, ".orphans")

	// ensure the top‐level .orphaned directory exists
	if err := os.MkdirAll(orphansRoot, 0755); err != nil {
		return fmt.Errorf("failed to create orphans root: %w", err)
	}

	for fileHash, fileRef := range ks.references {
		// check each chunk for presence on disk
		missingAny := false
		for _, chunk := range fileRef.Chunks {
			if chunk == nil {
				missingAny = true
				break
			}
			path := ks.chunkPath(chunk.Key)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				missingAny = true
				break
			}
		}

		if !missingAny {
			continue // this file is fully intact
		}

		// we have an orphaned file: prepare its own sub‐directory
		hexHash := fmt.Sprintf("%x", fileHash[:])
		fileOrphansDir := filepath.Join(orphansRoot, hexHash)
		if err := os.MkdirAll(fileOrphansDir, 0755); err != nil {
			fmt.Printf("Warning: could not make orphan dir %s: %v\n", fileOrphansDir, err)
			continue
		}

		// move each existing chunk file into that sub‐dir
		for _, chunk := range fileRef.Chunks {
			if chunk == nil {
				continue
			}
			srcPath := ks.chunkPath(chunk.Key)
			if _, err := os.Stat(srcPath); err == nil {
				// dest filename is the hex key with extension
				destName := fmt.Sprintf("%x%s", chunk.Key, FileExtension)
				dstPath := filepath.Join(fileOrphansDir, destName)
				if err := os.Rename(srcPath, dstPath); err != nil {
					fmt.Printf("Warning: failed to move chunk %x: %v\n", chunk.Key, err)
				}
			}
			// in all cases, drop it from the registry
			delete(ks.registry, chunk.Key)
		}

		// (optionally) move the metadata TOML into the same dir
		metaName := fmt.Sprintf("%s.toml", hexHash)
		srcMeta := filepath.Join(metadataDir, metaName)
		dstMeta := filepath.Join(orphansRoot, metaName)
		if err := os.Rename(srcMeta, dstMeta); err != nil {
			fmt.Printf("Warning: failed to move metadata %s: %v\n", metaName, err)
		}

		// finally, remove this file from the in‐memory map
		delete(ks.references, fileHash)

		fmt.Printf("Orphaned file %s → %s\n", hexHash, fileOrphansDir)
	}

	return nil
}
