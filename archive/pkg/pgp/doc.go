// Package pgp provides PGP signing and verification utilities.
// Used by the DataSync service to verify NVD data integrity
// and by the CVEDB service to sign/verify exported databases.
//
// Implementation uses golang.org/x/crypto/openpgp. If that package is not
// available (removed in x/crypto >= 0.17.0), use the stub implementation
// in pgp_stub.go by setting build tag "pgp_stub".
package pgp
