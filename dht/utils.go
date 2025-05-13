package dht

import "crypto/rand"

func bucketIndex(a, b NodeID) int {
	for i := range len(a) {
		x := a[i] ^ b[i]
		if x != 0 {
			for j := range 8 {
				if x&(0x80>>j) != 0 {
					return i*8 + j
				}
			}
		}
	}
	return 159
}

func xorDistance(a, b NodeID) [20]byte {
	var dist [20]byte
	for i := range 20 {
		dist[i] = a[i] ^ b[i]
	}
	return dist
}

func GenerateRandomID() NodeID {
	var id NodeID
	rand.Read(id[:])
	return id
}

func NodeInRange(id, start, end NodeID) bool {
	return CompareNodeIDs(id, start) >= 0 && CompareNodeIDs(id, end) < 0
}

func MaxNodeID() NodeID {
	var max NodeID
	for i := range max {
		max[i] = 0xFF
	}
	return max
}
