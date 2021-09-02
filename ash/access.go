package ash

// Access defines access levels.
type Access int

// The available access levels.
const (
	None Access = 0
	List Access = 1 << iota
	Find
	Create
	Update
	Delete
	Read  = List | Find
	Write = Create | Update | Delete
	Full  = Read | Write
)

var accessTags = map[rune]Access{
	' ': None,
	'L': List,
	'F': Find,
	'C': Create,
	'U': Update,
	'D': Delete,
	'R': Read,
	'W': Write,
	'*': Full,
}

// AccessTable defines a string based access table.
type AccessTable map[string]Access

// Collect will return all strings with a matching access level.
func (t AccessTable) Collect(match Access) []string {
	// collect matches
	list := make([]string, 0, len(t))
	for item, access := range t {
		if access&match != 0 {
			list = append(list, item)
		}
	}

	return list
}

// AccessMatrix defines a string based access matrix.
type AccessMatrix map[string][]string

// Compile return an access table for the provided column.
func (m AccessMatrix) Compile(column int) AccessTable {
	// prepare table
	table := make(AccessTable, len(m))

	// fill table
	for key, columns := range m {
		for _, char := range columns[column] {
			table[key] |= accessTags[char]
		}
	}

	return table
}

// NamedAccessMatrix defines a named string based access matrix.
type NamedAccessMatrix struct {
	Columns []string
	Matrix  AccessMatrix
}

// Compile return an access table for the provided column.
func (m NamedAccessMatrix) Compile(column string) AccessTable {
	// get index
	index := -1
	for i, key := range m.Columns {
		if key == column {
			index = i
			break
		}
	}

	return m.Matrix.Compile(index)
}
