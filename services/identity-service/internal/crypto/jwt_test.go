package crypto

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
    "os"
    "path/filepath"
    "testing"
    "time"
)

func setupJWTManager(t *testing.T) *JWTManager {
    t.Helper()

    // Generate test RSA key pair
    privKey, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        t.Fatalf("generate RSA key: %v", err)
    }

    dir := t.TempDir()
    privPath := filepath.Join(dir, "private.pem")
    pubPath := filepath.Join(dir, "public.pem")

    // Write private key
    privPEM := pem.EncodeToMemory(&pem.Block{
        Type:  "RSA PRIVATE KEY",
        Bytes: x509.MarshalPKCS1PrivateKey(privKey),
    })
    os.WriteFile(privPath, privPEM, 0600)

    // Write public key
    pubBytes, _ := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
    pubPEM := pem.EncodeToMemory(&pem.Block{
        Type:  "PUBLIC KEY",
        Bytes: pubBytes,
    })
    os.WriteFile(pubPath, pubPEM, 0644)

    mgr, err := NewJWTManager(privPath, pubPath, "test-key", "auth-service", "api", 15*time.Minute)
    if err != nil {
        t.Fatalf("NewJWTManager: %v", err)
    }
    return mgr
}

func TestJWT_SignAndParse(t *testing.T) {
    mgr := setupJWTManager(t)

    tokenStr, jti, exp, err := mgr.Sign("user-123", "user", []string{"scan:read"})
    if err != nil {
        t.Fatalf("Sign(): %v", err)
    }
    if tokenStr == "" || jti == "" {
        t.Error("Sign() returned empty values")
    }
    if exp.Before(time.Now()) {
        t.Error("ExpiresAt should be in the future")
    }

    claims, err := mgr.Parse(tokenStr)
    if err != nil {
        t.Fatalf("Parse(): %v", err)
    }
    if claims.Subject != "user-123" {
        t.Errorf("Subject = %s, want user-123", claims.Subject)
    }
    if claims.Role != "user" {
        t.Errorf("Role = %s, want user", claims.Role)
    }
    if claims.ID != jti {
        t.Errorf("JTI mismatch: %s != %s", claims.ID, jti)
    }
}

func TestJWT_InvalidToken(t *testing.T) {
    mgr := setupJWTManager(t)
    _, err := mgr.Parse("not.a.valid.token")
    if err != ErrTokenInvalid {
        t.Errorf("Expected ErrTokenInvalid, got: %v", err)
    }
}
