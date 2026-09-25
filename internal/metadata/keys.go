package metadata

import (
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	PrefixObjects = "/vault/objects/"
	PrefixChunks  = "/vault/chunks/"
	PrefixNodes   = "/vault/nodes/"
)

// EncodeObjectKey creates a deterministic, URL-safe base64 string for an object key.
func EncodeObjectKey(key string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(key))
}

// DecodeObjectKey decodes a base64 encoded object key.
func DecodeObjectKey(encoded string) (string, error) {
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// ObjectKey returns the etcd key path for an object.
func ObjectKey(key string) string {
	return PrefixObjects + EncodeObjectKey(key)
}

// ChunkKey returns the etcd key path for a chunk.
func ChunkKey(chunkID string) string {
	return PrefixChunks + chunkID
}

// NodeKey returns the etcd key path for a node.
func NodeKey(nodeID string) string {
	return PrefixNodes + nodeID
}

// ValidateKey checks that an object key is well-formed.
func ValidateKey(key string) error {
	if strings.TrimSpace(key) == "" {
		return fmt.Errorf("object key cannot be empty")
	}
	if strings.Contains(key, "..") {
		return fmt.Errorf("object key contains illegal path traversal pattern: '..'")
	}
	return nil
}
