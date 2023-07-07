package stick

// P is a shorthand function to get a pointer of the value.
func P[T any](id T) *T {
	return &id
}

// Z is a shorthand to get a zero value of the specified type.
func Z[T any]() T {
	var z T
	return z
}

// N is a shorthand to get a typed nil pointer of the specified type.
func N[T any]() *T {
	return nil
}
