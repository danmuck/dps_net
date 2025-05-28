package key_store

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

// this is a reference to locally stored chunks of a file
// includes a pointer to the top level files metadata for reconstruction
// each file reference maps a chunk of the data to its storage location on disk
type FileChunk struct {
	Key       [KeySize]byte  `toml:"key"`
	FileName  string         `toml:"file_name"`
	Size      uint32         `toml:"chunk_size"`
	FileIndex uint32         `toml:"chunk_index"`
	Location  string         `toml:"location"`
	Protocol  string         `toml:"protocol"`
	DataHash  [HashSize]byte `toml:"data_hash"`
	// MetaData  *MetaData      `toml:"metadata,omitempty"`
}

// ////
// returns the file path for given key in the form
// keystore.storageDir/[key].fileExt
// //
func (ks *KeyStore) chunkPath(id [KeySize]byte) string {
	return filepath.Join(ks.storageDir, fmt.Sprintf("%x%s", id, FileExtension))
}

// ////
// write the data associated with a reference to disk
// and update the reference with the keystore in memory
// //
func (ks *KeyStore) WriteReferenceToDisk(ref *FileChunk, data []byte) error {
	ks.lock.Lock()
	defer ks.lock.Unlock()

	if uint32(len(data)) != ref.Size {
		return fmt.Errorf("chunk %d data size (%d) doesn't match reference size (%d)",
			ref.FileIndex, len(data), ref.Size)
	}

	// calculate data hash
	ref.DataHash = sha256.Sum256(data)

	// create chunk file
	chunkPath := ks.chunkPath(ref.Key)
	if err := os.WriteFile(chunkPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write chunk file: %w", err)
	}

	// verify the written data immediately
	writtenData, err := os.ReadFile(chunkPath)
	if err != nil {
		return fmt.Errorf("failed to verify written chunk: %w", err)
	}

	// verify size
	if len(writtenData) != len(data) {
		return fmt.Errorf("written chunk size mismatch: got %d, expected %d",
			len(writtenData), len(data))
	}

	// verify hash of written data
	writtenHash := sha256.Sum256(writtenData)
	if writtenHash != ref.DataHash {
		return fmt.Errorf("chunk data verification failed after write:\nstored:  %x\nwritten: %x",
			ref.DataHash, writtenHash)
	}

	ref.Location = chunkPath
	ref.Protocol = "file"

	// store the reference
	ks.registry[ref.Key] = *ref

	return nil
}

// ////
// load the data associated with a file reference from disk
// returns the payload as raw bytes
// //
func (ks *KeyStore) ReadDataFromDisk(key [KeySize]byte) ([]byte, error) {
	ks.lock.RLock()
	chunk, exists := ks.registry[key]
	ks.lock.RUnlock()

	if !exists {
		return nil, fmt.Errorf("chunk not found for key %x", key)
	}

	data, err := os.ReadFile(chunk.Location)
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk file: %w", err)
	}

	// verify data integrity
	dataHash := sha256.Sum256(data)
	if dataHash != chunk.DataHash {
		return nil, fmt.Errorf("chunk data corruption detected:\nstored hash:  %x\ncomputed hash: %x",
			chunk.DataHash, dataHash)
	}

	return data, nil
}

// ////
// lookup the file reference for a given key in memory
// //
func (ks *KeyStore) GetFileReference(key [KeySize]byte) (FileChunk, error) {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	chunk, exists := ks.registry[key]
	if !exists {
		return FileChunk{}, fmt.Errorf("chunk not found for key %x", key)
	}
	return chunk, nil
}

// ////
// delete the data associated with a key from disk
// and delete its reference from memory
// //
func (ks *KeyStore) deleteFileReferenceFromDisk(key [KeySize]byte) error {
	ks.lock.Lock()
	defer ks.lock.Unlock()

	chunk, exists := ks.registry[key]
	if !exists {
		return fmt.Errorf("chunk not found for key %x", key)
	}

	if err := os.Remove(chunk.Location); err != nil {
		return fmt.Errorf("failed to delete chunk file: %w", err)
	}

	delete(ks.registry, key)
	return nil
}
