package app

import (
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestKeyCipherEncryptsAndDecrypts(t *testing.T) {
	cipher, err := NewKeyCipher([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := cipher.Encrypt("openrouter-secret")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "openrouter-secret" || strings.Contains(encrypted, "openrouter") {
		t.Fatalf("encrypted payload leaked plaintext: %q", encrypted)
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "openrouter-secret" {
		t.Fatalf("decrypted = %q", decrypted)
	}
}

func TestKeyCipherRequires32Bytes(t *testing.T) {
	_, err := NewKeyCipher([]byte("short"))
	if !errors.Is(err, ErrEncryptionKeyRequired) {
		t.Fatalf("err = %v, want ErrEncryptionKeyRequired", err)
	}
}

func TestNewKeyCipherFromStringAcceptsRawBase64AndHex(t *testing.T) {
	raw := "12345678901234567890123456789012"
	values := []string{
		raw,
		base64.StdEncoding.EncodeToString([]byte(raw)),
		"base64:" + base64.StdEncoding.EncodeToString([]byte(raw)),
		hex.EncodeToString([]byte(raw)),
		"hex:" + hex.EncodeToString([]byte(raw)),
	}

	for _, value := range values {
		cipher, err := NewKeyCipherFromString(value)
		if err != nil {
			t.Fatalf("NewKeyCipherFromString(%q) error = %v", value, err)
		}
		encrypted, err := cipher.Encrypt("openrouter-secret")
		if err != nil {
			t.Fatalf("Encrypt(%q) error = %v", value, err)
		}
		decrypted, err := cipher.Decrypt(encrypted)
		if err != nil {
			t.Fatalf("Decrypt(%q) error = %v", value, err)
		}
		if decrypted != "openrouter-secret" {
			t.Fatalf("decrypted = %q", decrypted)
		}
	}
}
