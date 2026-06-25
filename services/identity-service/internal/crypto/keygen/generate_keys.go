// Package keygen provides a utility to generate RSA key pairs for JWT signing.
// Usage: go run ./internal/crypto/keygen/generate_keys.go
// Output: jwt_private.pem, jwt_public.pem in current directory
package main

import (
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/pem"
    "fmt"
    "os"
)

func main() {
    fmt.Println("Generating 4096-bit RSA key pair for JWT RS256...")

    privateKey, err := rsa.GenerateKey(rand.Reader, 4096)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error generating key: %v\n", err)
        os.Exit(1)
    }

    // Save private key
    privFile, _ := os.OpenFile("jwt_private.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
    pem.Encode(privFile, &pem.Block{
        Type:  "RSA PRIVATE KEY",
        Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
    })
    privFile.Close()
    fmt.Println("✓ Saved jwt_private.pem (keep secret, only in auth-service)")

    // Save public key
    pubBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
    pubFile, _ := os.OpenFile("jwt_public.pem", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
    pem.Encode(pubFile, &pem.Block{
        Type:  "PUBLIC KEY",
        Bytes: pubBytes,
    })
    pubFile.Close()
    fmt.Println("✓ Saved jwt_public.pem (share with all services)")
    fmt.Println("\nMount these into containers via Docker secrets or Kubernetes secrets.")
}
