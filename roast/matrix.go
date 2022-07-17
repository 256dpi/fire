package roast

import "reflect"

// Query represents an item query.
type Query func(Item) bool

// Item is single combination of dimensions.
type Item map[string]any

// Is will check if the specified dimension value matches.
func (i Item) Is(name string, value any) bool {
	return reflect.DeepEqual(i[name], value)
}

// Match will return whether the item matches one of provided queries.
func (i Item) Match(queries ...Query) bool {
	// evaluate queries
	for _, query := range queries {
		if query(i) {
			return true
		}
	}

	return false
}

func (i Item) copy() Item {
	item := Item{}
	for k, v := range i {
		item[k] = v
	}
	return item
}

type ignore struct{}

// Ignore may be returned by generate to ignore an item.
var Ignore = any(ignore{})

// Matrix provides a facility for matrix testing.
type Matrix struct {
	names []string
	items map[string][]Item
}

// New returns a new matrix.
func New() *Matrix {
	return &Matrix{
		items: map[string][]Item{},
	}
}

// Bool will generate a boolean dimension.
func (m *Matrix) Bool(name string) {
	m.Values(name, true, false)
}

// Values will generate a dimension using the specified values.
func (m *Matrix) Values(name string, values ...any) {
	m.Generate(name, values, nil)
}

// Generate will generate a dimension using the specified values and generator.
// The generator may be absent to just add the provided values. If the generator
// returns Ignore the value will be skipped.
func (m *Matrix) Generate(name string, values []any, fn func(value any, item Item) any) {
	// ensure values
	if len(values) == 0 {
		values = []any{nil}
	}

	// handle first dimension
	if len(m.names) == 0 {
		m.names = append(m.names, name)
		for _, value := range values {
			if fn != nil {
				value = fn(value, Item{})
			}
			m.items[name] = append(m.items[name], Item{
				name: value,
			})
		}
		return
	}

	// get base dimension
	base := m.items[m.names[len(m.names)-1]]

	// prepare new items
	newItems := make([]Item, 0, len(base)+len(values))

	// generate items
	for _, item := range base {
		for _, value := range values {
			if fn != nil {
				value = fn(value, item)
			}
			if value != Ignore {
				newItem := item.copy()
				newItem[name] = value
				newItems = append(newItems, newItem)
			}
		}
	}

	// set new items
	m.names = append(m.names, name)
	m.items[name] = newItems
}

// Items will return a dimension's items that match at least on of the specified
// queries. If no queries are specified the full list is returned. Queries
// are parsed using the Common Expression Language (CEL).
func (m *Matrix) Items(name string, queries ...Query) []Item {
	if len(queries) == 0 {
		return m.items[name]
	}
	var items []Item
	for _, item := range m.items[name] {
		if item.Match(queries...) {
			items = append(items, item)
		}
	}
	return items
}
