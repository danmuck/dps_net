package api

import (
	"crypto/rand"
	"fmt"

	"golang.org/x/crypto/blake2b"
)

// ///////////////////////////////// ///////////// /////
//
// # Key Constants and Conversion Functions
//
// //// //
const (
	// KeyBytes is the length of a NodeID in bytes
	KeyBytes = 20
	KeyBits  = KeyBytes * 8

	// DataHashBytes is the length of a DataHash in bytes
	// 	-- for data integrity
	// NOTE: blake2b / sha256 / sha512 ??
	DataHashBytes = 32

	// ApplicationIDBytes is the length of an AppID hash in bytes
	// 	-- for checking application/plugin integrity
	// sha512
	ApplicationIDBytes = 64
)

type NodeID [KeyBytes]byte
type DataHash [DataHashBytes]byte
type Address string

// Application Hash for version integrity
//
//	-- AppLock used for running different instances of an app/plugin
//		ie. Hash the AppID with an access key
type AppID [ApplicationIDBytes]byte
type AppLock [ApplicationIDBytes]byte

// ////
// SliceToNodeID converts a byte slice to a NodeID
// //
func SliceToNodeID(data []byte) (NodeID, error) {
	if len(data) != KeyBytes {
		return NodeID{}, fmt.Errorf("expected %d bytes, got %d", KeyBytes, len(data))
	}
	var id NodeID
	copy(id[:], data)
	return id, nil
}

// ////
// SliceToDataID converts a byte slice to a DataID
// //
func SliceToDataID(data []byte) (DataHash, error) {
	if len(data) != DataHashBytes {
		return DataHash{}, fmt.Errorf("expected %d bytes, got %d", DataHashBytes, len(data))
	}
	var id DataHash
	copy(id[:], data)
	return id, nil
}

// ////
// Blake2Conv converts a byte slice to a NodeID using Blake2b
// //
func Blake2Conv(data []byte) ([KeyBytes]byte, error) {
	h, err := blake2b.New(KeyBytes, nil)
	if err != nil {
		return [KeyBytes]byte{}, err
	}
	h.Write(data)

	var out [KeyBytes]byte
	copy(out[:], h.Sum(nil))
	return out, nil
}

// ///////////////////////////////// ///////////// /////
//
// Kademlia Helper Functions
//
// //// //

// ////
// GenerateRandomNodeID generates a random NodeID
// //
func GenerateRandomNodeID() NodeID {
	var id NodeID
	rand.Read(id[:])
	return id
}

// ////
// XorDistance calculates the XOR distance between two NodeIDs
// //
func XorDistance(a, b NodeID) NodeID {
	var dist NodeID
	for i := range KeyBytes {
		dist[i] = a[i] ^ b[i]
	}
	return dist
}

// ////
// KBucketIndex calculates the natural bucket index for a given NodeID
// //
func KBucketIndex(local, other NodeID) int {
	for i := range KeyBytes {
		x := local[i] ^ other[i]
		if x != 0 {
			for j := range 8 {
				if x&(0x80>>j) != 0 {
					return (KeyBits - 1) - (i*8 + j)
				}
			}
		}
	}
	return 0
}

// ////
// GetBit checks if a bit is set in a NodeID
// //
func GetBit(key NodeID, bit int) int {
	if bit < 0 || bit >= KeyBits {
		return 0
	}
	byteIndex := KeyBytes - 1 - (bit / 8)
	bitIndex := bit % 8

	if key[byteIndex]&(1<<bitIndex) != 0 {
		return 1
	}
	return 0
}

// ////
// SetBit sets a bit in a NodeID
// //
func SetBit(key NodeID, bit int) NodeID {
	if bit < 0 || bit >= KeyBits {
		return key
	}
	byteIndex := KeyBytes - 1 - (bit / 8)
	bitIndex := bit % 8

	key[byteIndex] |= 1 << bitIndex
	return key
}

// ////
// ClearBit clears a bit in a NodeID
// //
func ClearBit(key NodeID, bit int) NodeID {
	if bit < 0 || bit >= KeyBits {
		return key
	}
	byteIndex := KeyBytes - 1 - (bit / 8)
	bitIndex := bit % 8

	key[byteIndex] &^= 1 << bitIndex
	return key
}

// ////
// FlipBit flips a bit in a NodeID
// //
func FlipBit(key NodeID, bit int) NodeID {
	if bit < 0 || bit >= KeyBits {
		return key
	}
	byteIndex := KeyBytes - 1 - (bit / 8)
	bitIndex := bit % 8

	key[byteIndex] ^= 1 << bitIndex
	return key
}

// ////
// ConfirmPrefix checks if two NodeIDs have the same prefix of a given length
// //
func ConfirmPrefix(a, b NodeID, prefixLen int) bool {
	if prefixLen < 0 || prefixLen > KeyBits {
		return false
	}
	for i := range prefixLen {
		if GetBit(a, i) != GetBit(b, i) {
			return false
		}
	}
	return true
}
