// Package secretbox seals small secrets (such as third-party API keys) for
// storage using AES-256-GCM with a key derived from the application secret.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

const (
	prefix    = "v1."
	minSecret = 32
	info      = "dua-os/secretbox/v1"
)

var ErrOpen = errors.New("secretbox: cannot open sealed value")

type Box struct{ aead cipher.AEAD }

// New derives a 256-bit key from secret (>= 32 chars) with HKDF-SHA256.
func New(secret string) (*Box, error) {
	if len(secret) < minSecret {
		return nil, fmt.Errorf("secretbox: secret must be at least %d characters", minSecret)
	}
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, []byte(secret), nil, []byte(info)), key); err != nil {
		return nil, fmt.Errorf("secretbox: derive key: %w", err)
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

// Seal encrypts plaintext. aad binds the ciphertext to its owner (e.g. the
// workspace id) so a stolen row cannot be replayed under another owner.
func (b *Box) Seal(plaintext, aad string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("secretbox: nonce: %w", err)
	}
	sealed := b.aead.Seal(nonce, nonce, []byte(plaintext), []byte(aad))
	return prefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (b *Box) Open(sealed, aad string) (string, error) {
	if len(sealed) <= len(prefix) || sealed[:len(prefix)] != prefix {
		return "", ErrOpen
	}
	raw, err := base64.RawURLEncoding.DecodeString(sealed[len(prefix):])
	if err != nil || len(raw) < b.aead.NonceSize()+b.aead.Overhead() {
		return "", ErrOpen
	}
	n := b.aead.NonceSize()
	plain, err := b.aead.Open(nil, raw[:n], raw[n:], []byte(aad))
	if err != nil {
		return "", ErrOpen
	}
	return string(plain), nil
}
