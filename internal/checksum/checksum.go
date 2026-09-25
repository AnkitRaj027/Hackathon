package checksum

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// ComputeBytes calculates the SHA-256 hex string of a byte slice.
func ComputeBytes(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ComputeReader calculates the SHA-256 hex string of an io.Reader and returns the hex digest and total byte count.
func ComputeReader(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return "", 0, fmt.Errorf("failed calculating checksum: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// Verify returns true if expected and actual checksums match case-insensitively.
func Verify(expectedHex, actualHex string) bool {
	if len(expectedHex) != len(actualHex) {
		return false
	}
	for i := 0; i < len(expectedHex); i++ {
		e := expectedHex[i]
		a := actualHex[i]
		if e >= 'A' && e <= 'Z' {
			e += 32
		}
		if a >= 'A' && a <= 'Z' {
			a += 32
		}
		if e != a {
			return false
		}
	}
	return true
}
