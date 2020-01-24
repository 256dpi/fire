package heat

import (
	"crypto/sha256"

	"golang.org/x/crypto/pbkdf2"
)

// Secret wraps a bytes secret to allow key derivation.
type Secret []byte

// Derive will derive a key using the provided string.
func (s Secret) Derive(str string) Secret {
	return s.DeriveBytes([]byte(str))
}

// DeriveBytes will derive a key using the provided bytes.
func (s Secret) DeriveBytes(bytes []byte) Secret {
	return pbkdf2.Key(s, bytes, 4096, 32, sha256.New)
}
