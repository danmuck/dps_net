package key_store

import "fmt"

func (ks *KeyStore) StoreKey(key [KeySize]byte, value []byte) error {
	ks.lock.Lock()
	defer ks.lock.Unlock()
	ks.registry[key] = NewFileChunk(value, "net")
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
