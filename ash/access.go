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
