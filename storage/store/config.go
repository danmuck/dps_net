package key_store

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
)

const (
	KeySize       = 20      // 160 bits for kademlia dht routing
	HashSize      = 32      // 256 bits (sha-256) for data integrity
	CryptoSize    = 64      // 512 bits (sha-512) for security
	MinChunkSize  = 1 << 16 // 64kb minimum chunk
	MaxChunkSize  = 1 << 22 // 4mb maximum chunk
	TargetChunks  = 1000    // aim for ~1000 chunks for large files
	FileExtension = ".kdht"
	PRINT_CHUNKS  = 500

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
// returns the file path for given key in the form
// keystore.storageDir/[key].fileExt
// //
func (ks *KeyStore) chunkPath(id [KeySize]byte) string {
	return filepath.Join(ks.storage, fmt.Sprintf("%x%s", id, FileExtension))
}

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

func NewChunkID(
	chunkData []byte,
	fileHash [HashSize]byte,
	index uint64,
) [KeySize]byte {
	if chunkData != nil {
		// content‐addressed: SHA-1 of the raw chunk
		return sha1.Sum(chunkData)
	}

	// deterministic: SHA-1 of (fileHash || chunkIndex)
	buf := make([]byte, len(fileHash)+8)
	copy(buf, fileHash[:])
	binary.LittleEndian.PutUint64(buf[len(fileHash):], index)
	return sha1.Sum(buf)
}

func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Alloc = %v MiB", m.Alloc/1024/1024)
	fmt.Printf("\tTotalAlloc = %v MiB", m.TotalAlloc/1024/1024)
	fmt.Printf("\tSys = %v MiB", m.Sys/1024/1024)
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

// calculate optimal chunk size based on file size
func CalculateChunkSize(fileSize uint64) uint32 {
	// for small files, use minimum chunk size
	if fileSize < uint64(MinChunkSize) {
		return uint32(fileSize)
	}

	// calculate chunk size to achieve target number of chunks
	chunkSize := fileSize / uint64(TargetChunks)

	// round to nearest power of 2 for efficiency
	power := math.Log2(float64(chunkSize))
	chunkSize = uint64(math.Pow(2, math.Round(power)))

	// clamp to min/max sizes
	if chunkSize < uint64(MinChunkSize) {
		return MinChunkSize
	}
	if chunkSize > uint64(MaxChunkSize) {
		return MaxChunkSize
	}

	return uint32(chunkSize)
}

// ////
// read data from disk and compute its sha256 hash
// returns hash in raw bytes and the data size
func HashFile(filePath string) ([HashSize]byte, int64, error) {
	// open the file
	file, err := os.Open(filePath)
	if err != nil {
		return [HashSize]byte{}, 0, err
	}
	defer file.Close()

	// get the file size
	fileInfo, err := file.Stat()
	if err != nil {
		return [32]byte{}, 0, err
	}
	fileSize := fileInfo.Size()

	// create a new sha256 hash
	hasher := sha256.New()

	// read the file in chunks and update the hash
	bufSize := hasher.BlockSize()
	buf := make([]byte, bufSize)
	for {
		n, err := file.Read(buf)
		if err != nil && err != io.EOF {
			return [HashSize]byte{}, 0, err
		}
		if n == 0 {
			break
		}
		hasher.Write(buf[:n]) // update hash with the read chunk
	}

	// convert the hash to [HashSize]byte
	var hash [HashSize]byte
	copy(hash[:], hasher.Sum(nil))

	// return the hash and file size
	return hash, fileSize, nil
}

// ////
// copy data on disk
// //
func copyLocalData(srcPath, dstPath string) error {
	// check if dstpath is a directory
	if fileInfo, err := os.Stat(dstPath); err == nil && fileInfo.IsDir() {
		// if it's a directory, append the source file's name to the destination path
		srcFileName := filepath.Base(srcPath)
		dstPath = filepath.Join(dstPath, srcFileName)
	}

	// open the source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// create or overwrite the destination file
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// copy the contents of the source file to the destination
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	return nil
}

func ValidateDataSHA256(a, b []byte) bool {
	hash1 := sha256.Sum256(a)
	hash2 := sha256.Sum256(b)
	return hash1 == hash2
}

// convert byte slice to key
func SliceToKey(b []byte) ([KeySize]byte, error) {
	if len(b) != KeySize {
		return [KeySize]byte{}, fmt.Errorf("invalid hash length: got %d, want %d", len(b), KeySize)
	}
	var arr [KeySize]byte
	copy(arr[:], b)
	return arr, nil
}

// computes the sha checksum of data and returns a slice
func ShaCheckSum(obj []byte, bytes int) []byte {
	switch bytes {
	case KeySize:
		sha_1 := sha1.Sum(obj)
		return sha_1[:]
	case HashSize:
		sha_1 := sha256.Sum256(obj)
		return sha_1[:]
	case CryptoSize:
		sha_1 := sha512.Sum512(obj)
		return sha_1[:]
	default:
		sha_1 := sha1.Sum(obj)
		return sha_1[:]
	}
}
