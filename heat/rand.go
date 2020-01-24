package heat

import (
	"crypto/rand"
	"io"
)

// Rand will return n secure random bytes.
func Rand(n int) ([]byte, error) {
	// read from random generator
	bytes := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, bytes)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// MustRand will call Rand and panic on errors.
func MustRand(n int) []byte {
	// generate bytes
	bytes, err := Rand(n)
	if err != nil {
		panic(err.Error())
	}

	return bytes
}
