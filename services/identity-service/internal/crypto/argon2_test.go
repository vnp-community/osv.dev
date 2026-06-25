package crypto

import (
    "strings"
    "testing"
)

func TestPasswordHasher_Hash(t *testing.T) {
    h := NewPasswordHasher()

    hash, err := h.Hash("securePassword123!")
    if err != nil {
        t.Fatalf("Hash() error: %v", err)
    }
    if !strings.HasPrefix(hash, "$argon2id$") {
        t.Errorf("Hash() should start with $argon2id$, got: %s", hash)
    }
}

func TestPasswordHasher_TooShort(t *testing.T) {
    h := NewPasswordHasher()
    _, err := h.Hash("short")
    if err != ErrPasswordTooShort {
        t.Errorf("expected ErrPasswordTooShort, got: %v", err)
    }
}

func TestPasswordHasher_Verify_Correct(t *testing.T) {
    h := NewPasswordHasher()
    password := "securePassword123!"
    hash, _ := h.Hash(password)

    ok, err := h.Verify(password, hash)
    if err != nil {
        t.Fatalf("Verify() error: %v", err)
    }
    if !ok {
        t.Error("Verify() should return true for correct password")
    }
}

func TestPasswordHasher_Verify_Wrong(t *testing.T) {
    h := NewPasswordHasher()
    hash, _ := h.Hash("securePassword123!")

    ok, err := h.Verify("wrongPassword123!", hash)
    if err != nil {
        t.Fatalf("Verify() error: %v", err)
    }
    if ok {
        t.Error("Verify() should return false for wrong password")
    }
}

func TestPasswordHasher_UniqueHashes(t *testing.T) {
    h := NewPasswordHasher()
    password := "securePassword123!"
    hash1, _ := h.Hash(password)
    hash2, _ := h.Hash(password)

    if hash1 == hash2 {
        t.Error("Hash() should produce different hashes (unique salt) each time")
    }
}
