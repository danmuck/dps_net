package key_store

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// ////
// metadata for a file reference stored in memory
// this is the metadata for the file itself as a whole
// //
type MetaData struct {
	FileHash    [HashSize]byte   `toml:"file_hash"`
	TotalSize   uint64           `toml:"total_size"`
	FileName    string           `toml:"file_name"`
	Modified    int64            `toml:"modified"`
	Replicate   uint32           `toml:"replicate"`
	Permissions uint32           `toml:"permissions"`
	Signature   [CryptoSize]byte `toml:"signature"`
	TTL         uint64           `toml:"ttl"`
	ChunkSize   uint32           `toml:"chunk_size"`
	TotalChunks uint32           `toml:"total_chunks"`
}

// ////
// prepare the metadata for a file using a cryptographic signature
// note: this is just a constructor for a MetaData
// it calculates the chunk size and assigns permissions
// but no logic pertaining to security lives here
// //
func PrepareMetaDataSecure(name string, data []byte, permissions int, signature [CryptoSize]byte) (metadata MetaData, e error) {
	if permissions == 0 {
		metadata.Permissions = R_USER | W_USER | R_GROUP | R_OTHER // default
	} else {
		metadata.Permissions = uint32(permissions)
	}
	metadata.TotalSize = uint64(len(data))
	metadata.TTL = 24 * 60 * 60 // 24 hrs in seconds
	metadata.FileName = name
	metadata.Modified = time.Now().UnixNano()
	// metadata.Replication = "Not Implemented"
	metadata.Signature = signature
	metadata.ChunkSize = CalculateChunkSize(metadata.TotalSize)
	metadata.TotalChunks = uint32((metadata.TotalSize + uint64(metadata.ChunkSize) - 1) / uint64(metadata.ChunkSize))

	return metadata, nil
}

// ////
// prepare the metadata for a file with no signature
// note: this is just a constructor for a MetaData
// it calculates the chunk size and assigns permissions
// //
func prepareMetaData(name string, data []byte, permissions int) (metadata MetaData, e error) {
	if permissions == 0 {
		metadata.Permissions = R_USER | W_USER | R_GROUP | R_OTHER // default
	} else {
		metadata.Permissions = uint32(permissions)
	}

	metadata.TotalSize = uint64(len(data))
	metadata.TTL = 24 * 60 * 60
	metadata.FileName = name
	metadata.Modified = time.Now().UnixNano()
	// metadata.Replication = "Not Implemented"
	metadata.ChunkSize = CalculateChunkSize(metadata.TotalSize)

	// calculate total chunks with proper rounding up
	metadata.TotalChunks = uint32((metadata.TotalSize + uint64(metadata.ChunkSize) - 1) / uint64(metadata.ChunkSize))

	return metadata, nil
}

// ////
// writes the metadata file to disk, this is the map for the file
// and where its chunk references are stored
// TODO: should probably try to remove the reference metadata as they are written
// to disk, currently the metadata is stored on every chunk which seems unnecessary
// //
func (ks *KeyStore) WriteAllReferencesToDisk() error {
	refDir := filepath.Join(ks.storageDir, "metadata")
	if err := os.MkdirAll(refDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// save each complete file's metadata
	// 	including all references
	for hash, file := range ks.blueprints {
		filename := fmt.Sprintf("%x.toml", hash)
		path := filepath.Join(refDir, filename)

		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("failed to create metadata file: %w", err)
		}

		encoder := toml.NewEncoder(f)
		encoder.Indent = "  "

		if err := encoder.Encode(file); err != nil {
			f.Close()
			return fmt.Errorf("failed to encode file data: %w", err)
		}
		f.Close()
	}

	return nil
}

// ////
// load a complete file's metadata from disk
// //
func (ks *KeyStore) ReadFileMetadataFromDisk(key [HashSize]byte) (*FileReference, error) {
	metadataDir := filepath.Join(ks.storageDir, "metadata")
	filepath := filepath.Join(metadataDir, fmt.Sprintf("%x.toml", key))

	var file *FileReference
	if _, err := toml.DecodeFile(filepath, file); err != nil {
		return nil, fmt.Errorf("failed to decode metadata file: %w", err)
	}

	return file, nil
}

// ////
// loads all complete files metadata from disk
// stores each file and file reference into memory
// NOTE: does not read file data from disk only metadata
// //
func (ks *KeyStore) LoadAllFileMetadata() error {
	metadataDir := filepath.Join(ks.storageDir, "metadata")

	// create directory if it doesn't exist
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// read directory entries
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		return fmt.Errorf("failed to read metadata directory: %w", err)
	}

	// process each .toml file
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".toml") {
			// extract hash from filename
			hashStr := strings.TrimSuffix(entry.Name(), ".toml")
			var fileHash [HashSize]byte
			hashBytes, err := hex.DecodeString(hashStr)
			if err != nil {
				return fmt.Errorf("invalid metadata filename %s: %w", entry.Name(), err)
			}
			copy(fileHash[:], hashBytes)

			// load file metadata
			file, err := ks.ReadFileMetadataFromDisk(fileHash)
			if err != nil {
				return fmt.Errorf("failed to load metadata for %s: %w", entry.Name(), err)
			}

			// add to in-memory maps
			ks.blueprints[fileHash] = file // store the complete file struct
			for _, ref := range file.Chunks {
				if ref != nil {
					ks.registry[ref.Key] = *ref
				}
			}
		}
	}

	return nil
}
