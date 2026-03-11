package database

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/dfgh012316/popofinder/internal/linebot/search"
	"github.com/jmoiron/sqlx"
)

type MedicalPersonnel struct {
	ID               int            `db:"id"`
	City             string         `db:"city"`
	Hospital         string         `db:"hospital"`
	Department       sql.NullString `db:"department"`
	Name             string         `db:"name"`
	Education        sql.NullString `db:"education"`
	University       sql.NullString `db:"university"`
	GraduationStatus string         `db:"graduation_status"`
}

type SearchStats struct {
	TotalCount  int
	CurrentPage int
	TotalPages  int
	HasMore     bool
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Search queries medical_personnel with ILIKE filtering and offset-based pagination.
func (r *Repository) Search(criteria search.Criteria, offset int) ([]MedicalPersonnel, SearchStats, error) {
	var (
		whereClauses []string
		args         []any
		argIdx       = 1
	)

	term := "%" + criteria.SearchTerm + "%"

	switch criteria.SearchType {
	case search.TypeName:
		whereClauses = append(whereClauses, fmt.Sprintf("name ILIKE $%d", argIdx))
	case search.TypeHospital:
		whereClauses = append(whereClauses, fmt.Sprintf("hospital ILIKE $%d", argIdx))
	case search.TypeDepartment:
		whereClauses = append(whereClauses, fmt.Sprintf("department ILIKE $%d", argIdx))
	}
	args = append(args, term)
	argIdx++

	if criteria.City != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("city = $%d", argIdx))
		args = append(args, *criteria.City)
		argIdx++
	}

	where := ""
	if len(whereClauses) > 0 {
		where = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count query
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM medical_personnel %s", where)
	var totalCount int
	if err := r.db.Get(&totalCount, countSQL, args...); err != nil {
		return nil, SearchStats{}, fmt.Errorf("repository: count: %w", err)
	}

	// Data query
	dataSQL := fmt.Sprintf(
		"SELECT id, city, hospital, department, name, education, university, graduation_status FROM medical_personnel %s LIMIT 10 OFFSET $%d",
		where, argIdx,
	)
	args = append(args, offset)

	var records []MedicalPersonnel
	if err := r.db.Select(&records, dataSQL, args...); err != nil {
		return nil, SearchStats{}, fmt.Errorf("repository: select: %w", err)
	}

	currentPage := offset/10 + 1
	totalPages := (totalCount + 9) / 10
	stats := SearchStats{
		TotalCount:  totalCount,
		CurrentPage: currentPage,
		TotalPages:  totalPages,
		HasMore:     (offset + 10) < totalCount,
	}

	return records, stats, nil
}
