package key_store

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
)

func (ks *KeyStore) bootstrapDirs() error {
	// create directories if they don't exist
	if err := os.MkdirAll(ks.storage, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// load metadata files
	metadataDir := filepath.Join(ks.storage, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}
	ks.metadata = metadataDir
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
// delete the data associated with a key from disk
// and delete its reference from memory
// //
func (ks *KeyStore) deleteDataFromDisk(key [KeySize]byte) error {
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

// ////
// store a complete file to disk and return its top level reference in memory
// TODO: needs to mirror that loads into the network
// //
// //
// //
// func (ks *KeyStore) WriteFileToNetwork(name string, fileData []byte, permissions int) (*FileReference, error)
func (ks *KeyStore) WriteFileToDisk(name string, fileData []byte, permissions int) (*FileReference, error) {
	// prepare metadata
	metadata, err := prepareMetaData(name, fileData, permissions)
	if err != nil {
		return nil, err
	}

	// calculate and store file hash
	// this is the complete, constructed file data hash
	metadata.FileHash = sha256.Sum256(fileData)

	// create file object to store metadata
	file := &FileReference{
		MetaData: metadata,
		Chunks:   make([]*FileChunk, metadata.TotalChunks),
	}

	// process file data into chunks
	var totalBytesProcessed uint64 = 0
	for i := uint32(0); i < metadata.TotalChunks; i++ {
		// calculate chunk boundaries
		startIdx := uint64(i) * uint64(metadata.ChunkSize)
		endIdx := min(startIdx+uint64(metadata.ChunkSize), metadata.TotalSize)

		chunkData := fileData[startIdx:endIdx]
		chunkSize := uint32(len(chunkData))

		// create filereference for this chunk
		chunk := FileChunk{
			FileName:  metadata.FileName,
			Size:      chunkSize,
			FileIndex: i,
			Protocol:  "file",
			// MetaData:  &metadata,
			DataHash: sha256.Sum256(chunkData),
		}

		// calculate chunk's dht routing id
		chunkIDData := append(metadata.FileHash[:], byte(i))
		chunkID := sha1.Sum(chunkIDData)
		chunk.Key = chunkID

		// store the chunk
		if err := ks.WriteChunkToDisk(&chunk, chunkData); err != nil {
			// cleanup any chunks we've already stored
			for j := uint32(0); j < i; j++ {
				if file.Chunks[j] != nil {
					ks.deleteDataFromDisk(file.Chunks[j].Key)
				}
			}
			return nil, fmt.Errorf("failed to store chunk %d: %w", i, err)
		}

		// store reference in file
		chunkRef := chunk // NOTE: not sure why im making this copy here
		file.Chunks[i] = &chunkRef

		// debug print after storing each chunk reference
		fmt.Printf("Added chunk reference %d: Key=%x, Size=%d\n", i, chunkRef.Key, chunkRef.Size)

		totalBytesProcessed += uint64(chunkSize)

		// debug output for progress
		if i%PRINT_CHUNKS == 0 || i == metadata.TotalChunks-1 {
			fmt.Printf("Stored chunk %d/%d (%.1f%%)\n",
				i+1, metadata.TotalChunks, float64(i+1)/float64(metadata.TotalChunks)*100)
		}
	}

	// verify total bytes processed
	if totalBytesProcessed != metadata.TotalSize {
		// cleanup all chunks on size mismatch
		for _, ref := range file.Chunks {
			if ref != nil {
				ks.deleteDataFromDisk(ref.Key)
			}
		}
		return nil, fmt.Errorf("processed bytes (%d) doesn't match file size (%d)",
			totalBytesProcessed, metadata.TotalSize)
	}

	fmt.Printf("\nValidating file before storage:\n")
	fmt.Printf("File name: %s\n", file.MetaData.FileName)
	fmt.Printf("Total chunks: %d\n", file.MetaData.TotalChunks)
	fmt.Printf("References count: %d\n", len(file.Chunks))

	for i, ref := range file.Chunks {
		if ref == nil {
			fmt.Printf("Warning: Reference %d is nil\n", i)
		} else if i%PRINT_CHUNKS == 0 || i == len(file.Chunks)-1 {
			fmt.Printf("Reference %d: Key=%x, Size=%d\n", i, ref.Key, ref.Size)
		}
	}
	// store the complete file with metadata and references
	if err := ks.updateReference(file); err != nil {
		// cleanup all chunks on failure
		for _, ref := range file.Chunks {
			if ref != nil {
				ks.deleteDataFromDisk(ref.Key)
			}
		}
		return nil, fmt.Errorf("failed to store file metadata: %w", err)
	}

	return file, nil
}
