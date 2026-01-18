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

// store file to memory and write metadata toml to file system
// NOTE: this does not write data to disk
// it writes the metadata file, encoded into toml
// that is used to lookup files stored on disk
// it contains the metadata for the file and its chunked references
func (ks *KeyStore) updateReference(file *FileReference) error {
	ks.lock.Lock()
	defer ks.lock.Unlock()
	for i, ref := range file.Chunks {
		if ref == nil {
			continue
		}
		// look up the “live” version in ks.registry
		stored, ok := ks.registry[ref.Key]
		if !ok {
			return fmt.Errorf("missing chunk %x in local data", ref.Key)
		}
		// overwrite so our TOML encoder sees the right Location, DataHash, etc.
		file.Chunks[i].Location = stored.Location
		file.Chunks[i].DataHash = stored.DataHash
		file.Chunks[i].Protocol = stored.Protocol
		file.Chunks[i].FileName = stored.FileName
		file.Chunks[i].Size = stored.Size
		file.Chunks[i].FileIndex = stored.FileIndex
	}
	// store in memory
	ks.references[file.MetaData.FileHash] = file

	// create metadata directory if it doesn't exist
	metadataDir := filepath.Join(ks.storage, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// create metadata file path
	filename := fmt.Sprintf("%x.toml", file.MetaData.FileHash)
	filepath := filepath.Join(metadataDir, filename)

	// open file for writing
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %w", err)
	}
	defer f.Close()

	// create toml encoder
	encoder := toml.NewEncoder(f)
	encoder.Indent = "    "

	// encode the complete file structure
	if err := encoder.Encode(file); err != nil {
		return fmt.Errorf("failed to encode file: %w", err)
	}

	return nil
}

// ////
// prepare the metadata for a file using a cryptographic signature
// note: this is just a constructor for a MetaData
// it calculates the chunk size and assigns permissions
// but no logic pertaining to security lives here
// //
func prepareMetaDataSecure(name string, data []byte, permissions int, signature [CryptoSize]byte) (metadata FileMetaData, e error) {
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
func prepareMetaData(name string, data []byte, permissions int) (metadata FileMetaData, e error) {
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
	refDir := filepath.Join(ks.storage, "metadata")
	if err := os.MkdirAll(refDir, 0755); err != nil {
		return fmt.Errorf("failed to create metadata directory: %w", err)
	}

	// save each complete file's metadata
	// 	including all references
	for hash, file := range ks.references {
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
// loads all complete files metadata from disk
// stores each file and file reference into memory
// NOTE: does not read file data from disk only metadata
// //
func (ks *KeyStore) LoadAllFileMetadata() error {
	metadataDir := filepath.Join(ks.storage, "metadata")

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
			ks.references[fileHash] = file // store the complete file struct
			for _, ref := range file.Chunks {
				if ref != nil {
					ks.registry[ref.Key] = ref
				}
			}
		}
	}

	return nil
}
