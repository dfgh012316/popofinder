package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/dfgh012316/popofinder/internal/linebot/search"
	"github.com/jmoiron/sqlx"
)

const selectColumns = "id, city, hospital, department, name, education, source, verification_status, source_url, source_checked_at"

// Columns compared with 臺 read as 台: ParseCriteria already turned 臺 into 台 in the
// search term, and stored data spells it both ways (docs/open-decisions.md OD-13).
const (
	nameMatchCol       = "translate(name, '臺', '台')"
	hospitalMatchCol   = "translate(hospital, '臺', '台')"
	departmentMatchCol = "translate(department, '臺', '台')"
)

// tieBreakOrder orders rows within a search (and within each TypeAll match group).
// Not id: blog rows were loaded first, so id order put the blog list ahead of every
// hospital-directory row (docs/open-decisions.md OD-12).
const tieBreakOrder = "department, name, id"

type MedicalPersonnel struct {
	ID                 int            `db:"id"`
	City               string         `db:"city"`
	Hospital           string         `db:"hospital"`
	Department         sql.NullString `db:"department"`
	Name               string         `db:"name"`
	Education          sql.NullString `db:"education"`
	Source             sql.NullString `db:"source"`
	VerificationStatus string         `db:"verification_status"`
	SourceURL          sql.NullString `db:"source_url"`
	SourceCheckedAt    sql.NullTime   `db:"source_checked_at"`
}

// MatchCounts breaks a TypeAll result count down by which field matched first.
type MatchCounts struct {
	Name       int
	Hospital   int
	Department int
}

// PageSize is how many results Search returns per offset.
const PageSize = 10

type SearchStats struct {
	TotalCount  int
	CurrentPage int
	TotalPages  int
	HasMore     bool
	MatchCounts MatchCounts // populated only for search.TypeAll
}

// DirectoryHospital is one hospital whose official doctor directory is crawled
// (source = 'hospital_directory'), with how many doctors it contributes.
type DirectoryHospital struct {
	City     string `db:"city"`
	Hospital string `db:"hospital"`
	Count    int    `db:"count"`
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Ping verifies the database connection is usable (readiness check).
func (r *Repository) Ping(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Search queries medical_personnel with ILIKE filtering, deterministic ordering, and offset-based pagination.
// Every term must match (docs/open-decisions.md OD-14): in TypeAll the name, the hospital
// or the department, otherwise the searched column. A term matches a hospital whose
// name contains any of search.HospitalPatterns(term). TypeAll rows are ranked 0 when a
// term matches the name, 1 when one matches the hospital, else 2 (department only).
func (r *Repository) Search(criteria search.Criteria, offset int) ([]MedicalPersonnel, SearchStats, error) {
	var args []any
	ilike := func(col, s string) string {
		args = append(args, "%"+s+"%")
		return fmt.Sprintf("%s ILIKE $%d", col, len(args))
	}
	hospitalMatch := func(term string) string {
		var ors []string
		for _, p := range search.HospitalPatterns(term) {
			ors = append(ors, ilike(hospitalMatchCol, p))
		}
		return "(" + strings.Join(ors, " OR ") + ")"
	}

	var whereClauses, nameMatches, hospitalMatches []string
	for _, t := range criteria.Terms() {
		switch criteria.SearchType {
		case search.TypeAll:
			name, hospital := ilike(nameMatchCol, t), hospitalMatch(t)
			nameMatches = append(nameMatches, name)
			hospitalMatches = append(hospitalMatches, hospital)
			whereClauses = append(whereClauses, "("+name+" OR "+hospital+" OR "+ilike(departmentMatchCol, t)+")")
		case search.TypeHospital:
			whereClauses = append(whereClauses, hospitalMatch(t))
		case search.TypeDepartment:
			whereClauses = append(whereClauses, ilike(departmentMatchCol, t))
		}
	}
	matchRankExpr := "0"
	if criteria.SearchType == search.TypeAll {
		matchRankExpr = "CASE WHEN " + strings.Join(nameMatches, " OR ") + " THEN 0 WHEN " + strings.Join(hospitalMatches, " OR ") + " THEN 1 ELSE 2 END"
	}

	if criteria.City != nil {
		args = append(args, *criteria.City)
		whereClauses = append(whereClauses, fmt.Sprintf("city = $%d", len(args)))
	}

	where := ""
	if len(whereClauses) > 0 {
		where = "WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Count query
	countSQL := fmt.Sprintf(
		"SELECT COUNT(*) AS total, "+
			"COUNT(*) FILTER (WHERE %[1]s = 0) AS name_count, "+
			"COUNT(*) FILTER (WHERE %[1]s = 1) AS hospital_count, "+
			"COUNT(*) FILTER (WHERE %[1]s = 2) AS department_count "+
			"FROM medical_personnel %[2]s",
		matchRankExpr, where,
	)
	var counts struct {
		Total      int `db:"total"`
		Name       int `db:"name_count"`
		Hospital   int `db:"hospital_count"`
		Department int `db:"department_count"`
	}
	if err := r.db.Get(&counts, countSQL, args...); err != nil {
		return nil, SearchStats{}, fmt.Errorf("repository: count: %w", err)
	}
	totalCount := counts.Total

	// Data query
	orderBy := tieBreakOrder
	if criteria.SearchType == search.TypeAll {
		orderBy = matchRankExpr + ", " + tieBreakOrder
	}
	dataSQL := fmt.Sprintf(
		"SELECT "+selectColumns+" FROM medical_personnel %s ORDER BY %s LIMIT %d OFFSET $%d",
		where, orderBy, PageSize, len(args)+1,
	)
	args = append(args, offset)

	var records []MedicalPersonnel
	if err := r.db.Select(&records, dataSQL, args...); err != nil {
		return nil, SearchStats{}, fmt.Errorf("repository: select: %w", err)
	}

	currentPage := offset/PageSize + 1
	totalPages := (totalCount + PageSize - 1) / PageSize
	stats := SearchStats{
		TotalCount:  totalCount,
		CurrentPage: currentPage,
		TotalPages:  totalPages,
		HasMore:     (offset + PageSize) < totalCount,
	}
	if criteria.SearchType == search.TypeAll {
		stats.MatchCounts = MatchCounts{Name: counts.Name, Hospital: counts.Hospital, Department: counts.Department}
	}

	return records, stats, nil
}

// Where a search came from, for search_log.channel (OD-17).
const (
	ChannelLINE = "line"
	ChannelWeb  = "web"
)

// LogSearch records one user-initiated search for coverage analysis. It deliberately
// stores no user identifier (docs/open-decisions.md OD-5); channel is ChannelLINE or ChannelWeb.
func (r *Repository) LogSearch(ctx context.Context, criteria search.Criteria, resultCount int, channel string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO search_log (search_type, city, search_term, result_count, channel) VALUES ($1, $2, $3, $4, $5)",
		string(criteria.SearchType), criteria.City, criteria.SearchTerm, resultCount, channel,
	)
	if err != nil {
		return fmt.Errorf("repository: log search: %w", err)
	}
	return nil
}

// DirectoryHospitals lists the hospitals with crawled official directories, largest first.
func (r *Repository) DirectoryHospitals(ctx context.Context) ([]DirectoryHospital, error) {
	var hs []DirectoryHospital
	err := r.db.SelectContext(ctx, &hs,
		"SELECT city, hospital, COUNT(*) AS count FROM medical_personnel WHERE source = 'hospital_directory' GROUP BY city, hospital ORDER BY count DESC, hospital",
	)
	if err != nil {
		return nil, fmt.Errorf("repository: directory hospitals: %w", err)
	}
	return hs, nil
}
