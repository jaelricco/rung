// Package secret seals the one thing this server stores on a user's behalf
// that it must never hold in the clear: that user's own model provider key.
//
// The sealing key comes from the environment, not the database, so a dump of
// the database — a backup left somewhere, a stolen volume — carries no usable
// keys with it.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

type Box struct {
	aead cipher.AEAD
}

// New reads a 32-byte key written as base64 or hex. Generate one with:
//
//	openssl rand -base64 32
func New(encoded string) (*Box, error) {
	encoded = strings.TrimSpace(encoded)
	if encoded == "" {
		return nil, errors.New("the sealing key is empty")
	}

	key, err := decodeKey(encoded)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("the sealing key decodes to %d bytes, want 32", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

func decodeKey(encoded string) ([]byte, error) {
	if key, err := base64.StdEncoding.DecodeString(encoded); err == nil {
		return key, nil
	}
	if key, err := base64.RawURLEncoding.DecodeString(encoded); err == nil {
		return key, nil
	}
	if key, err := hex.DecodeString(encoded); err == nil {
		return key, nil
	}
	return nil, errors.New("the sealing key is neither base64 nor hex")
}

// Seal returns nonce || ciphertext. A fresh nonce per call is what keeps two
// users who happen to hold the same key from being visibly identical.
func (b *Box) Seal(plain string) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, []byte(plain), nil), nil
}

// Open reverses Seal. A key sealed under a different AI_CREDENTIALS_KEY fails
// here rather than being handed to a provider as garbage.
func (b *Box) Open(sealed []byte) (string, error) {
	size := b.aead.NonceSize()
	if len(sealed) < size {
		return "", errors.New("the stored value is too short to be sealed data")
	}
	plain, err := b.aead.Open(nil, sealed[:size], sealed[size:], nil)
	if err != nil {
		return "", errors.New("the stored key could not be unsealed: is AI_CREDENTIALS_KEY the one it was sealed with?")
	}
	return string(plain), nil
}
