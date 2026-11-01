package search

import "strings"

// SearchType represents the type of search to perform.
type SearchType string

const (
	// TypeAll matches name, hospital, or department (default when no @ prefix).
	TypeAll        SearchType = "all"
	TypeHospital   SearchType = "hospital"
	TypeDepartment SearchType = "department"
)

// Criteria holds parsed search parameters extracted from the user's message.
type Criteria struct {
	SearchType SearchType
	SearchTerm string  // words separated by single spaces; each must match
	City       *string // nil means all cities
}

// Terms returns the words of SearchTerm, or one empty term (matching everything)
// when it has none.
func (c Criteria) Terms() []string {
	if words := strings.Fields(c.SearchTerm); len(words) > 0 {
		return words
	}
	return []string{""}
}
