package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var ErrEncryptionKeyRequired = errors.New("encryption key required")

type KeyCipher struct {
	aead cipher.AEAD
}

func NewKeyCipher(key []byte) (KeyCipher, error) {
	if len(key) != 32 {
		return KeyCipher{}, fmt.Errorf("%w: expected 32 bytes", ErrEncryptionKeyRequired)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return KeyCipher{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return KeyCipher{}, err
	}
	return KeyCipher{aead: aead}, nil
}

func (c KeyCipher) Encrypt(plainText string) (string, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := c.aead.Seal(nonce, nonce, []byte(plainText), nil)
	return base64.RawStdEncoding.EncodeToString(sealed), nil
}

func (c KeyCipher) Decrypt(payload string) (string, error) {
	raw, err := base64.RawStdEncoding.DecodeString(payload)
	if err != nil {
		return "", err
	}
	if len(raw) < c.aead.NonceSize() {
		return "", errors.New("encrypted payload too short")
	}
	nonce := raw[:c.aead.NonceSize()]
	cipherText := raw[c.aead.NonceSize():]
	plainText, err := c.aead.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return "", err
	}
	return string(plainText), nil
}
