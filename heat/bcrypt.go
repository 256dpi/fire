package heat

import "golang.org/x/crypto/bcrypt"

// Hash uses bcrypt to safely compute a hash. The returned hash can be converted
// to readable string.
func Hash(str string) ([]byte, error) {
	// generate hash from password
	hash, err := HashBytes([]byte(str))
	if err != nil {
		return nil, err
	}

	return hash, nil
}

// HashBytes uses bcrypt to safely compute a hash. The returned hash can be
// converted to readable string.
func HashBytes(bytes []byte) ([]byte, error) {
	return bcrypt.GenerateFromPassword(bytes, bcrypt.DefaultCost)
}

// MustHash will call Hash and panic on errors.
func MustHash(str string) []byte {
	// hash string
	hash, err := Hash(str)
	if err != nil {
		panic(err)
	}

	return hash
}

// MustHashBytes will call HashBytes and panic on errors.
func MustHashBytes(bytes []byte) []byte {
	// hash bytes
	hash, err := HashBytes(bytes)
	if err != nil {
		panic(err)
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
	return bcrypt.CompareHashAndPassword(hash, bytes)
}
