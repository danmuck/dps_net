package key_store

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
)

type CacheEntry struct {
	data  []byte
	ts    time.Time
	timer time.Timer
}

type Cache struct {
	storage map[[KeySize]byte]*CacheEntry
}

type KeyStore struct {
	registry   map[[KeySize]byte]FileChunk
	blueprints map[[HashSize]byte]*FileReference
	cache      map[*FileReference]map[uint32]*FileChunk
	storageDir string

	lock sync.RWMutex
}

func InitKeyStore(storageDir string) (*KeyStore, error) {
	ks := &KeyStore{
		registry:   make(map[[KeySize]byte]FileChunk),
		blueprints: make(map[[HashSize]byte]*FileReference),
		storageDir: storageDir,
	}

	// create directories if they don't exist
	if err := os.MkdirAll(storageDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// load metadata files
	metadataDir := filepath.Join(storageDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create metadata directory: %w", err)
	}

	entries, err := os.ReadDir(metadataDir)
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
			var file FileReference
			if _, err := toml.DecodeFile(filepath.Join(metadataDir, entry.Name()), &file); err != nil {
				fmt.Printf("Warning: failed to decode file %s: %v\n", entry.Name(), err)
				continue
			}

			// store file in memory
			ks.blueprints[fileHash] = &file

			// also store chunk references
			for _, ref := range file.Chunks {
				if ref != nil {
					ks.registry[ref.Key] = *ref
				}
			}
		}
	}

	// verify chunks and handle orphaned metadata
	if err := ks.verifyFileReferences(); err != nil {
		fmt.Printf("Warning: error during chunk verification: %v\n", err)
	}

	return ks, nil
}

// store file to memory and write metadata toml to file system
// NOTE: this does not write data to disk
// it writes the metadata file, encoded into toml
// that is used to lookup files stored on disk
// it contains the metadata for the file and its chunked references
func (ks *KeyStore) fileToMemory(file *FileReference) error {
	ks.lock.Lock()
	defer ks.lock.Unlock()
	for i, ref := range file.Chunks {
		if ref == nil {
			continue
		}
		// look up the “live” version in ks.registry
		stored, ok := ks.registry[ref.Key]
		if !ok {
			return fmt.Errorf("missing chunk %x in registry", ref.Key)
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
	ks.blueprints[file.MetaData.FileHash] = file

	// create metadata directory if it doesn't exist
	metadataDir := filepath.Join(ks.storageDir, "metadata")
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

// return a pointer to a copy of file from memory
// NOTE: this does not include file data
// it is only the mapping to a files storage location
// and metadata for reconstruction
func (ks *KeyStore) FileFromMemory(key [HashSize]byte) (*FileReference, error) {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	file, exists := ks.blueprints[key]
	if !exists {
		return nil, fmt.Errorf("file not found for hash %x", key)
	}
	fmt.Printf("Loaded file metadata from %s\n", file.ShortString())
	fmt.Printf("Number of references: %d\n", len(file.Chunks))
	for i, ref := range file.Chunks {
		if ref != nil && (i%PRINT_CHUNKS == 0 || i == len(file.Chunks)-1) {
			fmt.Printf("Reference %d: Key=%x, DataHash=%x\n",
				i, ref.Key, ref.DataHash)
		}
	}
	// return a copy to prevent concurrent modification issues
	fileCopy := *file
	return &fileCopy, nil
}

// return copies in slice of all file references
func (ks *KeyStore) ListStoredFileReferences() []FileChunk {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	chunks := make([]FileChunk, 0, len(ks.registry))
	for _, chunk := range ks.registry {
		chunks = append(chunks, chunk) // copy by value
	}
	return chunks
}

// return copies in slice
func (ks *KeyStore) ListKnownFiles() []MetaData {
	ks.lock.RLock()
	defer ks.lock.RUnlock()

	entries := make([]MetaData, 0, len(ks.blueprints))
	for _, file := range ks.blueprints {
		entries = append(entries, file.MetaData) // copy the metadata from the file struct
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
	metadataDir := filepath.Join(ks.storageDir, "metadata")
	if err := os.RemoveAll(metadataDir); err != nil {
		return fmt.Errorf("failed to delete metadata directory: %w", err)
	}

	// reset the maps
	ks.registry = make(map[[KeySize]byte]FileChunk)
	ks.blueprints = make(map[[HashSize]byte]*FileReference) // changed to *file
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
	metadataDir := filepath.Join(ks.storageDir, "metadata")
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
	ks.registry = make(map[[KeySize]byte]FileChunk)
	ks.blueprints = make(map[[HashSize]byte]*FileReference)
	return nil
}

func (ks *KeyStore) moveToCache(sourcePath string) error {
	// create cache directory
	cacheDir := filepath.Join(ks.storageDir, ".cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// get filename and create destination path
	fileName := filepath.Base(sourcePath)
	destPath := filepath.Join(cacheDir, fileName)

	// move the file
	if err := os.Rename(sourcePath, destPath); err != nil {
		return fmt.Errorf("failed to move file to cache: %w", err)
	}

	fmt.Printf("Moved to cache: %s\n", fileName)
	return nil
}

func (ks *KeyStore) verifyFileReferences() error {
	orphanedFiles := make(map[string]bool)
	fmt.Printf("Verifying file references ... \n")

	// first pass: identify orphaned metadata files
	for key := range ks.registry {
		chunkPath := ks.chunkPath(key)
		if _, err := os.Stat(chunkPath); os.IsNotExist(err) {
			// find the metadata file containing this chunk reference
			metadataDir := filepath.Join(ks.storageDir, "metadata")
			entries, err := os.ReadDir(metadataDir)
			if err != nil {
				fmt.Printf("Warning: Failed to read metadata directory: %v\n", err)
				continue
			}

			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".toml") {
					orphanedFiles[entry.Name()] = true
				}
			}

			// remove this chunk from the in-memory map
			delete(ks.registry, key)
		}
	}

	// second pass: move orphaned files to cache
	if len(orphanedFiles) > 0 {
		metadataDir := filepath.Join(ks.storageDir, "metadata")
		for fileName := range orphanedFiles {
			sourcePath := filepath.Join(metadataDir, fileName)
			if err := ks.moveToCache(sourcePath); err != nil {
				fmt.Printf("Warning: %v\n", err)
			}
		}
	}

	return nil
}
