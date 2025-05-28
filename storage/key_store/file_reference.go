package key_store

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// the top level reference to a file in memory
// includes the metadata for reconstructing the file
// as well all file references
type FileReference struct {
	MetaData MetaData     `toml:"metadata"`
	Chunks   []*FileChunk `toml:"references,omitempty"`
}

const (
	R_USER = 0400 // read permission for owner
	W_USER = 0200 // write permission for owner
	X_USER = 0100 // execute permission for owner

	R_GROUP = 0040 // read permission for group
	W_GROUP = 0020 // write permission for group
	X_GROUP = 0010 // execute permission for group

	R_OTHER = 0004 // read permission for others
	W_OTHER = 0002 // write permission for others
	X_OTHER = 0001 // execute permission for others
)

// ////
// store a complete file to disk and return its top level reference in memory
// TODO: needs to mirror that loads into the network
// //
// //
// //
// func (ks *KeyStore) WriteFileToNetwork(name string, fileData []byte, permissions int) (*FileReference, error)
func (ks *KeyStore) WriteFileDataToDisk(name string, fileData []byte, permissions int) (*FileReference, error) {
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
		if err := ks.WriteReferenceToDisk(&chunk, chunkData); err != nil {
			// cleanup any chunks we've already stored
			for j := uint32(0); j < i; j++ {
				if file.Chunks[j] != nil {
					ks.deleteFileReferenceFromDisk(file.Chunks[j].Key)
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
				ks.deleteFileReferenceFromDisk(ref.Key)
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
	if err := ks.fileToMemory(file); err != nil {
		// cleanup all chunks on failure
		for _, ref := range file.Chunks {
			if ref != nil {
				ks.deleteFileReferenceFromDisk(ref.Key)
			}
		}
		return nil, fmt.Errorf("failed to store file metadata: %w", err)
	}

	return file, nil
}

// ////
// reassemble a file from disk
// NOTE: this requires the complete file and its data to be local
// all of its references must be stored locally
//
// returns the reconstructed file in raw bytes
// //
func (ks *KeyStore) ReassembleFileFromDisk(key [HashSize]byte) ([]byte, error) {
	// get the complete file record
	file, err := ks.FileFromMemory(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get file metadata: %w", err)
	}

	// pre-allocate the complete file buffer
	fileData := make([]byte, file.MetaData.TotalSize)
	var bytesWritten uint64 = 0

	// read and verify each chunk using stored references
	for i, ref := range file.Chunks {
		if ref == nil {
			return nil, fmt.Errorf("missing chunk reference at index %d", i)
		}

		// read chunk from disk
		chunkData, err := ks.ReadDataFromDisk(ref.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to read chunk %d: %w", i, err)
		}

		// verify chunk size
		if uint32(len(chunkData)) != ref.Size {
			return nil, fmt.Errorf("chunk %d size mismatch: got %d, expected %d",
				i, len(chunkData), ref.Size)
		}

		// verify chunk integrity
		dataHash := sha256.Sum256(chunkData)
		if dataHash != ref.DataHash {
			return nil, fmt.Errorf("chunk %d data corruption detected", i)
		}

		// copy chunk data to correct position
		startIdx := uint64(i) * uint64(file.MetaData.ChunkSize)
		copy(fileData[startIdx:], chunkData)
		bytesWritten += uint64(len(chunkData))

		// progress reporting
		if i%100 == 0 || i == int(file.MetaData.TotalChunks-1) {
			fmt.Printf("Reassembled chunk %d/%d (%.1f%%)\n",
				i+1, file.MetaData.TotalChunks,
				float64(i+1)/float64(file.MetaData.TotalChunks)*100)
		}
	}

	// verify total bytes reassembled
	if bytesWritten != file.MetaData.TotalSize {
		return nil, fmt.Errorf("size mismatch: wrote %d bytes, expected %d",
			bytesWritten, file.MetaData.TotalSize)
	}

	// verify final file integrity
	fileHash := sha256.Sum256(fileData)
	if fileHash != file.MetaData.FileHash {
		return nil, fmt.Errorf("reassembled file hash mismatch")
	}

	return fileData, nil
}

// ////
// writes the reconstructed file to the given path
// NOTE: this requires the complete file and its data to be local
// all of its references must be stored locally
// //
func (ks *KeyStore) CopyFileToPath(key [HashSize]byte, outputPath string) error {
	// get the complete file record
	file, err := ks.FileFromMemory(key)
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	// create output file
	f, err := os.OpenFile(outputPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer f.Close()

	var bytesWritten uint64 = 0
	hasher := sha256.New()

	// process chunks using stored references
	for i, ref := range file.Chunks {
		if ref == nil {
			return fmt.Errorf("missing chunk reference at index %d", i)
		}

		// get chunk data
		chunkData, err := ks.ReadDataFromDisk(ref.Key)
		if err != nil {
			return fmt.Errorf("failed to read chunk %d: %w", i, err)
		}

		// verify chunk size
		expectedSize := ref.Size
		if uint32(len(chunkData)) != expectedSize {
			return fmt.Errorf("chunk %d size mismatch: got %d, expected %d",
				i, len(chunkData), expectedSize)
		}

		// verify chunk integrity
		dataHash := sha256.Sum256(chunkData)
		if dataHash != ref.DataHash {
			return fmt.Errorf("chunk %d data corruption detected: stored hash %x, computed hash %x",
				i, ref.DataHash, dataHash)
		}

		// write chunk to file
		n, err := f.Write(chunkData)
		if err != nil {
			return fmt.Errorf("failed to write chunk %d: %w", i, err)
		}

		// update hash
		hasher.Write(chunkData)

		bytesWritten += uint64(n)

		// progress reporting
		if i%100 == 0 || i == int(file.MetaData.TotalChunks-1) {
			fmt.Printf("Wrote chunk %d/%d (%.1f%%) - size=%d bytes\n",
				i+1, file.MetaData.TotalChunks,
				float64(i+1)/float64(file.MetaData.TotalChunks)*100,
				n)
		}
	}

	// verify total size
	if bytesWritten != file.MetaData.TotalSize {
		return fmt.Errorf("size mismatch: wrote %d bytes, expected %d",
			bytesWritten, file.MetaData.TotalSize)
	}

	// flush the file to ensure all data is written
	if err := f.Sync(); err != nil {
		return fmt.Errorf("failed to flush file: %w", err)
	}

	// close the file before verifying
	f.Close()

	// verify final file integrity
	reassembledHash, length, err := HashFile(outputPath)
	if err != nil {
		return fmt.Errorf("failed to hash reassembled file: %w", err)
	}

	if length != int64(file.MetaData.TotalSize) {
		return fmt.Errorf("final size mismatch: got %d, expected %d",
			length, file.MetaData.TotalSize)
	}

	if reassembledHash != file.MetaData.FileHash {
		return fmt.Errorf("final hash mismatch:\n  got:      %x\n  expected: %x",
			reassembledHash, file.MetaData.FileHash)
	}

	return nil
}

// ////
// load a file from disk and distribute
func (ks *KeyStore) LoadAndStoreFile(localFilePath string) (*FileReference, error) {
	// open the file
	f, err := os.Open(localFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file: %w", err)
	}
	defer f.Close()

	// get file info for size
	fileInfo, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// calculate file hash using streaming
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return nil, fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// reset file pointer
	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to reset file position: %w", err)
	}

	// prepare metadata
	fileName := filepath.Base(localFilePath)
	metadata := MetaData{
		FileName:    fileName,
		TotalSize:   uint64(fileInfo.Size()),
		Modified:    time.Now().UnixNano(),
		Permissions: uint32(fileInfo.Mode().Perm()),
		ChunkSize:   CalculateChunkSize(uint64(fileInfo.Size())),
	}
	var fileHash [HashSize]byte
	copy(fileHash[:], hash.Sum(nil))
	metadata.FileHash = fileHash
	// copy(metadata.filehash[:], hash.sum(nil))
	metadata.TotalChunks = uint32((metadata.TotalSize + uint64(metadata.ChunkSize) - 1) / uint64(metadata.ChunkSize))

	// create file reference
	file := &FileReference{
		MetaData: metadata,
		Chunks:   make([]*FileChunk, metadata.TotalChunks),
	}

	fmt.Printf("Starting chunking process:\n")
	fmt.Printf("Total size: %d bytes\n", metadata.TotalSize)
	fmt.Printf("Chunk size: %d bytes\n", metadata.ChunkSize)
	fmt.Printf("Expected chunks: %d\n", metadata.TotalChunks)

	// process file in chunks
	buffer := make([]byte, metadata.ChunkSize)
	var totalBytesRead uint64 = 0

	for i := uint32(0); i < metadata.TotalChunks; i++ {
		// calculate expected chunk size
		var bytesToRead uint32 = metadata.ChunkSize
		if i == metadata.TotalChunks-1 {
			// for the last chunk, calculate remaining bytes
			remainingBytes := metadata.TotalSize - totalBytesRead
			bytesToRead = uint32(remainingBytes)
			fmt.Printf("Last chunk %d: Reading remaining %d bytes\n", i, bytesToRead)
		}

		// read chunk
		n, err := io.ReadFull(f, buffer[:bytesToRead])
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return nil, fmt.Errorf("failed to read chunk %d: %w", i, err)
		}

		if n == 0 {
			return nil, fmt.Errorf("unexpected end of file at chunk %d", i)
		}

		if i%100 == 0 || i == metadata.TotalChunks-1 {
			fmt.Printf("Chunk %d: Read %d bytes (total: %d/%d)\n",
				i, n, totalBytesRead+uint64(n), metadata.TotalSize)
		}
		chunkData := buffer[:n]

		// create filereference for this chunk
		chunk := FileChunk{
			FileName:  metadata.FileName,
			Size:      uint32(n),
			FileIndex: i,
			Protocol:  "file",
			// MetaData:  &metadata,
			DataHash: sha256.Sum256(chunkData),
		}

		// calculate chunk's dht routing id
		chunkIDData := append(metadata.FileHash[:], make([]byte, 8)...)
		binary.LittleEndian.PutUint64(chunkIDData[len(metadata.FileHash):], uint64(i))
		chunkID := sha1.Sum(chunkIDData)
		chunk.Key = chunkID

		// fmt.Println(chunk.String())
		// store the chunk
		if err := ks.WriteReferenceToDisk(&chunk, chunkData); err != nil {
			// cleanup on failure
			for j := uint32(0); j < i; j++ {
				if file.Chunks[j] != nil {
					ks.deleteFileReferenceFromDisk(file.Chunks[j].Key)
				}
			}
			return nil, fmt.Errorf("failed to store chunk %d: %w", i, err)
		}

		// store reference in file
		chunkRef := chunk // make a copy
		file.Chunks[i] = &chunkRef

		totalBytesRead += uint64(n)

		// progress reporting
		if i%100 == 0 || i == metadata.TotalChunks-1 {
			PrintMemUsage()
			fmt.Printf("Stored chunk %d/%d (%.1f%%) - size: %d bytes\n",
				i+1, metadata.TotalChunks,
				float64(i+1)/float64(metadata.TotalChunks)*100,
				n)
		}
	}

	// verify total bytes read
	if totalBytesRead != metadata.TotalSize {
		// cleanup on failure
		for _, ref := range file.Chunks {
			if ref != nil {
				ks.deleteFileReferenceFromDisk(ref.Key)
			}
		}
		return nil, fmt.Errorf("total bytes read (%d) doesn't match file size (%d)",
			totalBytesRead, metadata.TotalSize)
	}

	// final verification
	fmt.Printf("\n=== Final Verification ===\n")
	fmt.Printf("Total chunks stored: %d\n", len(file.Chunks))
	for i, ref := range file.Chunks {
		if ref == nil {
			return nil, fmt.Errorf("missing reference for chunk %d", i)
		}
		if i%PRINT_CHUNKS == 0 || i == len(file.Chunks)-1 {
			fmt.Printf("Chunk %d: Size=%d, Index=%d\n", i, ref.Size, ref.FileIndex)
		}
	}

	// fmt.Println(file.String())
	// store the complete file metadata
	if err := ks.fileToMemory(file); err != nil {
		// cleanup on failure
		for _, ref := range file.Chunks {
			if ref != nil {
				ks.deleteFileReferenceFromDisk(ref.Key)
			}
		}
		return nil, fmt.Errorf("failed to store file: %w", err)
	}

	return file, nil
}
