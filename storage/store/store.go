package key_store

import (
	"crypto/sha256"
	"fmt"
	"os"
)

func (ks *KeyStore) StoreKey(key [KeySize]byte, value []byte) error {
	ks.lock.Lock()
	defer ks.lock.Unlock()
	chunk := NewFileChunk(value, "net", "")
	ks.WriteChunkToDisk(chunk, value)

	return nil
}

// ////
// write the data associated with a reference to disk
// and update the reference with the keystore in memory
// //
func (ks *KeyStore) WriteChunkToDisk(ref *FileChunk, data []byte) error {
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

	ref.Protocol = "file"
	ref.Location = chunkPath

	// store the reference
	ks.registry[ref.Key] = ref

	return nil
}

// ////
// lookup the file chunk for a given key in memory
// //
func (ks *KeyStore) GetChunkLocation(key [KeySize]byte) (*FileChunk, error) {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	chunk, exists := ks.registry[key]
	if !exists {
		return nil, fmt.Errorf("chunk not found for key %x", key)
	}
	return chunk, nil
}

// return a pointer to a copy of file from memory
// NOTE: this does not include file data
// it is only the mapping to a files storage location
// and metadata for reconstruction
func (ks *KeyStore) ReferenceFromMemory(key [HashSize]byte) (*FileReference, error) {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	reference, exists := ks.references[key]
	if !exists {
		return nil, fmt.Errorf("file not found for hash %x", key)
	}
	fmt.Printf("Loaded file metadata from %s\n", reference.ShortString())
	fmt.Printf("Number of references: %d\n", len(reference.Chunks))

	// return a copy to prevent concurrent modification issues
	fileCopy := *reference
	return &fileCopy, nil
}
