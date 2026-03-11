package search

// SearchType represents the type of search to perform.
type SearchType string

const (
	TypeName       SearchType = "name"
	TypeHospital   SearchType = "hospital"
	TypeDepartment SearchType = "department"
)

// Criteria holds parsed search parameters extracted from the user's message.
type Criteria struct {
	SearchType SearchType
	SearchTerm string
	City       *string // nil means all cities
}
