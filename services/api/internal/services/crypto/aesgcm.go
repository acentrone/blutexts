// Package crypto provides envelope encryption for sensitive at-rest data
// (currently: GHL OAuth refresh tokens). Uses AES-256-GCM with a 32-byte
// key sourced from the ENCRYPTION_KEY env var.
//
// On-wire format (base64-encoded for column storage):
//
//	[ "v1:" prefix | 12-byte nonce | ciphertext+tag ]
//
// The "v1:" prefix lets us key-rotate later without ambiguity. If the prefix
// is missing on Decrypt, the value is assumed to be legacy plaintext (so
// deploys that haven't run the backfill yet keep working).
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const versionPrefix = "v1:"

// Encryptor wraps an AES-256-GCM AEAD. nil-safe: a nil receiver returns
// values unchanged, which lets the server boot in dev without an encryption
// key (with a loud warning at startup) while still working in prod.
type Encryptor struct {
	aead cipher.AEAD
}

// New constructs an Encryptor from a 64-character hex key (32 bytes after
// decoding). Returns nil with no error when key is empty — caller decides
// whether to fail-fast or run with at-rest encryption disabled.
func New(hexKey string) (*Encryptor, error) {
	if hexKey == "" {
		return nil, nil
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be hex-encoded: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes (64 hex chars); got %d bytes", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Encryptor{aead: aead}, nil
}

// Encrypt returns base64(versionPrefix || nonce || ciphertext+tag). When the
// receiver is nil (no key configured), returns the plaintext unchanged so
// dev environments keep working — production should always have a key.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if e == nil || plaintext == "" {
		return plaintext, nil
	}
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := e.aead.Seal(nil, nonce, []byte(plaintext), nil)
	return versionPrefix + base64.StdEncoding.EncodeToString(append(nonce, ct...)), nil
}

// Decrypt reverses Encrypt. Strings without the version prefix are
// assumed to be legacy plaintext and returned unchanged — this lets the
// transition land without a synchronous backfill (the backfill can run in
// the background and re-encrypt any bare values it finds).
func (e *Encryptor) Decrypt(stored string) (string, error) {
	if stored == "" {
		return "", nil
	}
	if !strings.HasPrefix(stored, versionPrefix) {
		// Legacy plaintext from before encryption-at-rest landed.
		return stored, nil
	}
	if e == nil {
		return "", fmt.Errorf("decrypt called with no encryption key configured")
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, versionPrefix))
	if err != nil {
		return "", fmt.Errorf("decrypt base64: %w", err)
	}
	nonceSize := e.aead.NonceSize()
	if len(raw) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ct := raw[:nonceSize], raw[nonceSize:]
	pt, err := e.aead.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(pt), nil
}
