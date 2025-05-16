package api

import (
	"bytes"
	"testing"
)

func TestConvertToNodeID(t *testing.T) {
	input := make([]byte, KeyBytes)
	for i := range KeyBytes {
		input[i] = byte(i)
	}
	id, err := SliceToNodeID(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(id[:], input) {
		t.Errorf("expected %v, got %v", input, id)
	}
}

func TestConvertToDataID(t *testing.T) {
	input := make([]byte, DataHashBytes)
	for i := range DataHashBytes {
		input[i] = byte(i)
	}
	id, err := SliceToDataID(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(id[:], input) {
		t.Errorf("expected %v, got %v", input, id)
	}
}

func TestXorDistance(t *testing.T) {
	var a, b NodeID
	for i := range KeyBytes {
		a[i] = 0xFF
	}
	dist := XorDistance(a, b)
	for i := range KeyBytes {
		if dist[i] != 0xFF {
			t.Errorf("expected 0xFF at byte %d, got 0x%X", i, dist[i])
		}
	}
}

func TestGetSetClearBit(t *testing.T) {
	var id NodeID
	id = SetBit(id, 0)
	if GetBit(id, 0) != 1 {
		t.Errorf("bit 0 should be set")
	}
	id = ClearBit(id, 0)
	if GetBit(id, 0) != 0 {
		t.Errorf("bit 0 should be cleared")
	}
}

func TestConfirmPrefix(t *testing.T) {
	// Identical keys
	a := GenerateRandomNodeID()
	b := a
	if !ConfirmPrefix(a, b, KeyBits) {
		t.Errorf("identical keys should have full prefix match\na: %08b\nb: %08b", a, b)
	}

	// Explicitly mismatch bit 0
	a = ClearBit(a, 0)
	b = SetBit(a, 0)
	if ConfirmPrefix(a, b, 1) {
		t.Errorf("should not match prefix after flipping MSB\na: %08b\nb: %08b", a, b)
	}
}

func BenchmarkXorDistance(b *testing.B) {
	a := GenerateRandomNodeID()
	c := GenerateRandomNodeID()
	b.ResetTimer()
	for range b.N {
		_ = XorDistance(a, c)
	}
}

func BenchmarkKBucketIndex(b *testing.B) {
	a := GenerateRandomNodeID()
	c := GenerateRandomNodeID()
	b.ResetTimer()
	for range b.N {
		_ = KBucketIndex(a, c)
	}
}

func BenchmarkGetBit(b *testing.B) {
	a := GenerateRandomNodeID()
	b.ResetTimer()
	for i := range b.N {
		_ = GetBit(a, i%KeyBits)
	}
}
