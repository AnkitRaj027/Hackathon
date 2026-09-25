package checksum

import (
	"bytes"
	"testing"
)

func TestComputeBytes(t *testing.T) {
	data := []byte("hello world")
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	actual := ComputeBytes(data)
	if actual != expected {
		t.Fatalf("expected %s, got %s", expected, actual)
	}
}

func TestComputeReader(t *testing.T) {
	data := []byte("vault distributed storage")
	r := bytes.NewReader(data)
	hash, n, err := ComputeReader(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != int64(len(data)) {
		t.Fatalf("expected %d bytes, got %d", len(data), n)
	}
	expected := ComputeBytes(data)
	if hash != expected {
		t.Fatalf("expected %s, got %s", expected, hash)
	}
}

func TestVerify(t *testing.T) {
	h := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if !Verify(h, h) {
		t.Fatal("expected identical hashes to match")
	}
	if !Verify("B94D27B9934D3E08A52E52D7DA7DABFAC484EFE37A5380EE9088F7ACE2EFCDE9", h) {
		t.Fatal("expected case-insensitive match")
	}
	if Verify(h, "0000000000000000000000000000000000000000000000000000000000000000") {
		t.Fatal("expected mismatched hashes to fail")
	}
}
