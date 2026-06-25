package crypto

import (
	"encoding/base64"
	"testing"
)

func TestAES256GCM(t *testing.T) {
	// 32-byte key base64 encoded
	keyStr := "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI=" // 32 bytes

	t.Run("ValidKey", func(t *testing.T) {
		_, err := NewAES256GCM(keyStr)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("InvalidKey_TooShort", func(t *testing.T) {
		shortKey := base64.StdEncoding.EncodeToString([]byte("short"))
		_, err := NewAES256GCM(shortKey)
		if err == nil {
			t.Fatal("expected error for short key, got nil")
		}
	})

	t.Run("InvalidKey_NotBase64", func(t *testing.T) {
		_, err := NewAES256GCM("not-base64-!!!")
		if err == nil {
			t.Fatal("expected error for invalid base64, got nil")
		}
	})

	t.Run("EncryptDecrypt_RoundTrip", func(t *testing.T) {
		cipher, err := NewAES256GCM(keyStr)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		plaintext := "my super secret password"
		encrypted, err := cipher.Encrypt(plaintext)
		if err != nil {
			t.Fatalf("encrypt failed: %v", err)
		}
		if encrypted == plaintext {
			t.Fatal("encrypted should not match plaintext")
		}

		decrypted, err := cipher.Decrypt(encrypted)
		if err != nil {
			t.Fatalf("decrypt failed: %v", err)
		}
		if decrypted != plaintext {
			t.Fatalf("expected %q, got %q", plaintext, decrypted)
		}
	})

	t.Run("Encrypt_EmptyString", func(t *testing.T) {
		cipher, _ := NewAES256GCM(keyStr)
		res, err := cipher.Encrypt("")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res != "" {
			t.Fatalf("expected empty result, got %q", res)
		}
	})

	t.Run("Decrypt_EmptyString", func(t *testing.T) {
		cipher, _ := NewAES256GCM(keyStr)
		res, err := cipher.Decrypt("")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res != "" {
			t.Fatalf("expected empty result, got %q", res)
		}
	})
}
