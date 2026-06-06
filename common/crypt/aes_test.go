package crypt

import (
	"testing"
)

func TestAesGcmRoundtrip(t *testing.T) {
	key := []byte("0123456789abcdef")
	plaintext := "Hello, v2node! This is a test message for AES-GCM encryption."
	encrypted, err := AesEncrypt([]byte(plaintext), key)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	if encrypted == plaintext {
		t.Fatal("Encrypted text should differ from plaintext")
	}
	decrypted, err := AesDecrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt error: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("Decrypted mismatch: got %q, want %q", decrypted, plaintext)
	}
}

func TestAesGcmWrongKey(t *testing.T) {
	key := []byte("0123456789abcdef")
	encrypted, err := AesEncrypt([]byte("secret"), key)
	if err != nil {
		t.Fatalf("Encrypt error: %v", err)
	}
	wrongKey := []byte("fedcba9876543210")
	_, err = AesDecrypt(encrypted, wrongKey)
	if err == nil {
		t.Fatal("Wrong key should have been rejected")
	}
}

func TestAesGcmShortCiphertext(t *testing.T) {
	key := []byte("0123456789abcdef")
	_, err := AesDecrypt("YWJj", key) // base64("abc") = 3 bytes, too short
	if err == nil {
		t.Fatal("Short ciphertext should have been rejected")
	}
}

func TestAesGcmEmptyPlaintext(t *testing.T) {
	key := []byte("0123456789abcdef")
	encrypted, err := AesEncrypt([]byte(""), key)
	if err != nil {
		t.Fatalf("Encrypt empty: %v", err)
	}
	decrypted, err := AesDecrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt empty: %v", err)
	}
	if decrypted != "" {
		t.Fatalf("Expected empty string, got %q", decrypted)
	}
}

func TestAesGcmLargeData(t *testing.T) {
	key := []byte("0123456789abcdef")
	data := make([]byte, 64*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	encrypted, err := AesEncrypt(data, key)
	if err != nil {
		t.Fatalf("Encrypt large: %v", err)
	}
	decrypted, err := AesDecrypt(encrypted, key)
	if err != nil {
		t.Fatalf("Decrypt large: %v", err)
	}
	if decrypted != string(data) {
		t.Fatal("Large data roundtrip mismatch")
	}
}

func TestAesGcmInvalidKey(t *testing.T) {
	shortKey := []byte("short")
	_, err := AesEncrypt([]byte("test"), shortKey)
	if err == nil {
		t.Fatal("Short key should have been rejected")
	}
}

func TestAesGcmInvalidBase64(t *testing.T) {
	key := []byte("0123456789abcdef")
	_, err := AesDecrypt("not!valid!base64!", key)
	if err == nil {
		t.Fatal("Invalid base64 should have been rejected")
	}
}
