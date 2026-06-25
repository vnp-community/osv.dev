// Package crypto provides AES-256-GCM encryption for sensitive credentials.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// AES256GCM provides authenticated encryption using AES-256-GCM.
// The encryption key must be exactly 32 bytes (256 bits).
type AES256GCM struct {
	key []byte
}

// NewAES256GCM creates an AES256GCM from a base64-encoded 32-byte key.
func NewAES256GCM(keyBase64 string) (*AES256GCM, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, errors.New("AES key must be exactly 32 bytes (256 bits)")
	}
	return &AES256GCM{key: key}, nil
}

// NewAES256GCMFromBytes creates an AES256GCM directly from a 32-byte key.
func NewAES256GCMFromBytes(key []byte) (*AES256GCM, error) {
	if len(key) != 32 {
		return nil, errors.New("AES key must be exactly 32 bytes (256 bits)")
	}
	keyCopy := make([]byte, 32)
	copy(keyCopy, key)
	return &AES256GCM{key: keyCopy}, nil
}

// Encrypt encrypts a plaintext string and returns a base64-encoded ciphertext.
// Returns empty string if plaintext is empty.
func (c *AES256GCM) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	// Prepend nonce to ciphertext
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext back to the plaintext string.
// Returns empty string if ciphertext is empty.
func (c *AES256GCM) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, cipherData := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}
