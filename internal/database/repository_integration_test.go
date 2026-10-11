//go:build integration

package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/dfgh012316/popofinder/internal/linebot/search"
	"github.com/stretchr/testify/require"
)

// openIntegrationDB connects to TEST_DATABASE_URL (set by `make test-integration`)
// and empties medical_personnel and its history so each test starts from known ids.
func openIntegrationDB(t *testing.T) *Repository {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL is required (run: make test-integration)")
	}
	db, err := Connect(url)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	db.MustExec("TRUNCATE medical_personnel, medical_personnel_history RESTART IDENTITY")
	return NewRepository(db)
}

// seedPersonnel inserts rows of {city, hospital, department, name} in order;
// a nil department is stored as NULL.
func seedPersonnel(t *testing.T, r *Repository, rows [][4]any) {
	t.Helper()
	for _, row := range rows {
		r.db.MustExec(
			"INSERT INTO medical_personnel (city, hospital, department, name) VALUES ($1, $2, $3, $4)",
			row[0], row[1], row[2], row[3],
		)
	}
}

func names(ps []MedicalPersonnel) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name
	}
	return out
}

// sharedSeed is fictional data; ids 1–6 follow this order. Hospital-match rows get
// smaller ids than name-match rows so ordering can only come from the match rank.
var sharedSeed = [][4]any{
	{"台北", "林口長庚", "內科", "王小明"},
	{"台北", "林新醫院", "牙科", "陳大文"},
	{"台中", "中山醫院", "新陳代謝科", "李四"},
	{"台北", "和平醫院", "家醫科", "林一"},
	{"高雄", "林園診所", nil, "張三"},
	{"台北", "測試診所", "小兒科", "林二"},
}

func TestSearchAllFieldsOrdersNameFirst(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, sharedSeed)

	got, stats, err := r.Search(search.Criteria{SearchType: search.TypeAll, SearchTerm: "林"}, 0)
	require.NoError(t, err)
	require.Equal(t, []string{"林一", "林二", "王小明", "陳大文", "張三"}, names(got))
	require.Equal(t, 5, stats.TotalCount)
	require.Equal(t, MatchCounts{Name: 2, Hospital: 3, Department: 0}, stats.MatchCounts)

	got, stats, err = r.Search(search.Criteria{SearchType: search.TypeAll, SearchTerm: "陳"}, 0)
	require.NoError(t, err)
	require.Equal(t, []string{"陳大文", "李四"}, names(got))
	require.Equal(t, MatchCounts{Name: 1, Hospital: 0, Department: 1}, stats.MatchCounts)
}

func TestSearchAllFieldsWithCity(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, sharedSeed)
	taipei := "台北"

	got, stats, err := r.Search(search.Criteria{SearchType: search.TypeAll, SearchTerm: "林", City: &taipei}, 0)
	require.NoError(t, err)
	require.Equal(t, []string{"林一", "林二", "王小明", "陳大文"}, names(got))
	require.Equal(t, MatchCounts{Name: 2, Hospital: 2, Department: 0}, stats.MatchCounts)
}

func TestSearchHospitalTypeOrdersByID(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, sharedSeed)

	got, stats, err := r.Search(search.Criteria{SearchType: search.TypeHospital, SearchTerm: "林"}, 0)
	require.NoError(t, err)
	require.Equal(t, []string{"王小明", "陳大文", "張三"}, names(got))
	require.Equal(t, 3, stats.TotalCount)
	require.Equal(t, MatchCounts{}, stats.MatchCounts)
}

func TestSearchPaginationStable(t *testing.T) {
	r := openIntegrationDB(t)
	rows := [][4]any{{"台北", "甲乙診所", "內科", "丙"}}
	for i := 1; i <= 12; i++ {
		rows = append(rows, [4]any{"台北", "某醫院", "內科", fmt.Sprintf("甲%02d", i)})
	}
	seedPersonnel(t, r, rows)
	criteria := search.Criteria{SearchType: search.TypeAll, SearchTerm: "甲"}

	page1, stats1, err := r.Search(criteria, 0)
	require.NoError(t, err)
	want1 := make([]string, 10)
	for i := range want1 {
		want1[i] = fmt.Sprintf("甲%02d", i+1)
	}
	require.Equal(t, want1, names(page1))
	require.True(t, stats1.HasMore)

	page2, stats2, err := r.Search(criteria, 10)
	require.NoError(t, err)
	require.Equal(t, []string{"甲11", "甲12", "丙"}, names(page2))
	require.False(t, stats2.HasMore)

	seen := map[string]bool{}
	for _, n := range append(names(page1), names(page2)...) {
		require.False(t, seen[n], "duplicate across pages: %s", n)
		seen[n] = true
	}
	require.Len(t, seen, 13)
}

func TestLogSearchInsertsRow(t *testing.T) {
	r := openIntegrationDB(t)
	r.db.MustExec("TRUNCATE search_log RESTART IDENTITY")
	ctx := context.Background()
	taipei := "台北"

	require.NoError(t, r.LogSearch(ctx, search.Criteria{SearchType: search.TypeAll, SearchTerm: "林", City: &taipei}, 4))
	require.NoError(t, r.LogSearch(ctx, search.Criteria{SearchType: search.TypeHospital, SearchTerm: "三總"}, 0))

	type logRow struct {
		SearchType  string         `db:"search_type"`
		City        sql.NullString `db:"city"`
		SearchTerm  string         `db:"search_term"`
		ResultCount int            `db:"result_count"`
		HasTS       bool           `db:"has_ts"`
	}
	var got []logRow
	require.NoError(t, r.db.Select(&got,
		"SELECT search_type, city, search_term, result_count, created_at IS NOT NULL AS has_ts FROM search_log ORDER BY id"))
	require.Equal(t, []logRow{
		{SearchType: "all", City: sql.NullString{String: "台北", Valid: true}, SearchTerm: "林", ResultCount: 4, HasTS: true},
		{SearchType: "hospital", City: sql.NullString{}, SearchTerm: "三總", ResultCount: 0, HasTS: true},
	}, got)
}

func TestSearchLogHasNoUserColumn(t *testing.T) {
	r := openIntegrationDB(t)

	var cols []string
	require.NoError(t, r.db.Select(&cols,
		"SELECT column_name FROM information_schema.columns WHERE table_name = 'search_log' ORDER BY ordinal_position"))
	require.Equal(t, []string{"id", "created_at", "search_type", "city", "search_term", "result_count"}, cols)
}

func TestSearchLogRejectsUnknownType(t *testing.T) {
	r := openIntegrationDB(t)

	_, err := r.db.Exec("INSERT INTO search_log (search_type, search_term, result_count) VALUES ('name', 'x', 0)")
	require.Error(t, err)
}

func TestSearchReturnsSourceCheckedAt(t *testing.T) {
	r := openIntegrationDB(t)
	r.db.MustExec("INSERT INTO medical_personnel (city, hospital, department, name, source, source_url, source_checked_at) VALUES ('台北', '甲醫院', '內科', '王一', 'hospital_directory', 'https://example.com/a', '2026-10-12'), ('台北', '甲醫院', '內科', '王二', 'blog', NULL, NULL)")

	got, _, err := r.Search(search.Criteria{SearchType: search.TypeAll, SearchTerm: "王"}, 0)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.True(t, got[0].SourceCheckedAt.Valid)
	require.Equal(t, "2026-10-12", got[0].SourceCheckedAt.Time.Format("2006-01-02"))
	require.Equal(t, "hospital_directory", got[0].Source.String)
	require.False(t, got[1].SourceCheckedAt.Valid)
}
