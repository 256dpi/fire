package heat

import (
	"github.com/256dpi/xo"
	"golang.org/x/crypto/bcrypt"
)

var hashCost = bcrypt.DefaultCost

// UnsafeFastHash can be called to set the minimum allowed hash cost. This
// should only be used for speeding up automated tests.
func UnsafeFastHash() {
	hashCost = bcrypt.MinCost
}

// Hash uses bcrypt to safely compute a hash. The returned hash can be converted
// to readable string.
func Hash(str string) ([]byte, error) {
	// generate hash from password
	hash, err := HashBytes([]byte(str))
	if err != nil {
		return nil, xo.W(err)
	}

	return hash, nil
}

// HashBytes uses bcrypt to safely compute a hash. The returned hash can be
// converted to readable string.
func HashBytes(bytes []byte) ([]byte, error) {
	buf, err := bcrypt.GenerateFromPassword(bytes, hashCost)
	return buf, xo.W(err)
}

// MustHash will call Hash and panic on errors.
func MustHash(str string) []byte {
	// hash string
	hash, err := Hash(str)
	if err != nil {
		panic(err.Error())
	}

	return hash
}

// MustHashBytes will call HashBytes and panic on errors.
func MustHashBytes(bytes []byte) []byte {
	// hash bytes
	hash, err := HashBytes(bytes)
	if err != nil {
		panic(err.Error())
	}

	return hash
}

// Compare will safely compare the specified hash to its unhashed version and
// return nil if they match.
func Compare(hash []byte, str string) error {
	return CompareBytes(hash, []byte(str))
}

// CompareBytes will safely compare the specified hash to its unhashed version
// and return nil if they match.
func CompareBytes(hash, bytes []byte) error {
	return xo.W(bcrypt.CompareHashAndPassword(hash, bytes))
}
