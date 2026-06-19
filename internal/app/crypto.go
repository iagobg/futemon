package app

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

var ErrEncryptionKeyRequired = errors.New("encryption key required")

type KeyCipher struct {
	aead cipher.AEAD
}

func NewKeyCipher(key []byte) (KeyCipher, error) {
	if len(key) != 32 {
		return KeyCipher{}, fmt.Errorf("%w: expected 32 bytes, got %d", ErrEncryptionKeyRequired, len(key))
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

func NewKeyCipherFromString(value string) (KeyCipher, error) {
	key, err := parseKeyCipherString(value)
	if err != nil {
		return KeyCipher{}, err
	}
	return NewKeyCipher(key)
}

func parseKeyCipherString(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "base64:") {
		return decodeKeyCipherBase64(strings.TrimPrefix(value, "base64:"))
	}
	if strings.HasPrefix(value, "hex:") {
		return decodeKeyCipherHex(strings.TrimPrefix(value, "hex:"))
	}
	if len(value) == 64 {
		if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == 32 {
			return decoded, nil
		}
	}
	if decoded, err := decodeKeyCipherBase64(value); err == nil && len(decoded) == 32 {
		return decoded, nil
	}
	return []byte(value), nil
}

func decodeKeyCipherBase64(value string) ([]byte, error) {
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	return base64.RawStdEncoding.DecodeString(value)
}

func decodeKeyCipherHex(value string) ([]byte, error) {
	return hex.DecodeString(value)
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
