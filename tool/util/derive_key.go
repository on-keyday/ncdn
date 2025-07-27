package util

import (
	"crypto/sha256"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

func DeriveKey(sharedKey []byte, info string) ([]byte, error) {
	aesKey := hkdf.New(sha256.New, sharedKey, nil, []byte(info)) // Ensure sharedKey is hashed to 32 bytes
	// 16 bytes for AES-128
	var aesKeyBytes [16]byte
	_, err := io.ReadFull(aesKey, aesKeyBytes[:])
	if err != nil {
		return nil, fmt.Errorf("failed to derive AES key: %w", err)
	}
	return aesKeyBytes[:], nil
}
