package app

import (
	"errors"
	"strings"
	"testing"
)

func TestKeyCipherEncryptsAndDecrypts(t *testing.T) {
	cipher, err := NewKeyCipher([]byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatal(err)
	}

	encrypted, err := cipher.Encrypt("gemini-secret")
	if err != nil {
		t.Fatal(err)
	}
	if encrypted == "gemini-secret" || strings.Contains(encrypted, "gemini") {
		t.Fatalf("encrypted payload leaked plaintext: %q", encrypted)
	}

	decrypted, err := cipher.Decrypt(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	if decrypted != "gemini-secret" {
		t.Fatalf("decrypted = %q", decrypted)
	}
}

func TestKeyCipherRequires32Bytes(t *testing.T) {
	_, err := NewKeyCipher([]byte("short"))
	if !errors.Is(err, ErrEncryptionKeyRequired) {
		t.Fatalf("err = %v, want ErrEncryptionKeyRequired", err)
	}
}
