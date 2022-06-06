package stick

// Registrable is value that can be registered with a Registry.
type Registrable interface {
	Validate() error
}

// Registry is a multi-key index of typed values.
type Registry[T Registrable] struct {
	indexer []func(T) string
	indexes []map[string]T
	list    []T
}

// NewRegistry will create and return a new registry using the specified index
// functions that must return unique keys.
func NewRegistry[T Registrable](values []T, indexer ...func(T) string) *Registry[T] {
	// created indexes
	indexes := make([]map[string]T, 0, len(indexer))
	for range indexer {
		indexes = append(indexes, map[string]T{})
	}

	// created registry
	r := &Registry[T]{
		indexer: indexer,
		indexes: indexes,
	}

	// add values
	r.Add(values...)

	return r
}

// Add will add the specified values to the registry.
func (r *Registry[T]) Add(values ...T) {
	for _, value := range values {
		// validate value
		err := value.Validate()
		if err != nil {
			panic("stick: invalid value: " + err.Error())
		}

		// index value
		for i, indexer := range r.indexer {
			// get key
			key := indexer(value)
			if key == "" {
				panic("stick: missing key")
			}

			// check index
			_, ok := r.indexes[i][key]
			if ok {
				panic("stick: value already added")
			}

			// add to index
			r.indexes[i][key] = value
		}

		// add to list
		r.list = append(r.list, value)
	}
}

// Get will attempt lookup a value using the specified predicate.
func (r *Registry[T]) Get(predicate T) (T, bool) {
	// check indexes
	for i, index := range r.indexes {
		value, ok := index[r.indexer[i](predicate)]
		if ok {
			return value, true
		}
	}

	// prepare zero value
	var value T

	return value, false
}

// MustGet will call Get and panic if not value has been found.
func (r *Registry[T]) MustGet(predicate T) T {
	// get value
	value, ok := r.Get(predicate)
	if !ok {
		panic("stick: missing value")
	}

	return value
}

// All will return a list of all added values.
func (r *Registry[T]) All() []T {
	// copy list
	list := make([]T, len(r.list))
	copy(list, r.list)

	return list
}
