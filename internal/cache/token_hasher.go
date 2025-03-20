// Package cache provides in-memory caching for user and role data
// with automatic expiration to minimize database lookups
package cache

import (
	"crypto/sha256"
	"encoding/hex"
)

// TokenHasher provides functionality for securely hashing tokens
// to use as cache keys without storing the raw token values
type TokenHasher struct{}

// NewTokenHasher creates a new token hasher
func NewTokenHasher() *TokenHasher {
	return &TokenHasher{}
}

// HashToken creates a secure hash of a token for use as a cache key
// The hash is deterministic so the same token always produces the same key
// without revealing the original token value
func (h *TokenHasher) HashToken(token string) string {
	// Create a new SHA-256 hasher
	hasher := sha256.New()
	
	// Write the token bytes to the hasher
	hasher.Write([]byte(token))
	
	// Get the hash and convert to hexadecimal string
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)
	
	return hashString
}
