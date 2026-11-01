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
// and empties medical_personnel, its history and takedown suppressions so each test starts from known ids.
func openIntegrationDB(t *testing.T) *Repository {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL is required (run: make test-integration)")
	}
	db, err := Connect(url)
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	db.MustExec("TRUNCATE medical_personnel, medical_personnel_history, takedown_suppressions RESTART IDENTITY")
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

func TestSearchHospitalTypeMatchCounts(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, sharedSeed)

	got, stats, err := r.Search(search.Criteria{SearchType: search.TypeHospital, SearchTerm: "林"}, 0)
	require.NoError(t, err)
	require.Equal(t, []string{"王小明", "陳大文", "張三"}, names(got))
	require.Equal(t, 3, stats.TotalCount)
	require.Equal(t, MatchCounts{}, stats.MatchCounts)
}

// Every search type orders by department, then name, then id — never by id alone, which
// would put rows loaded earlier (the blog list) ahead of later hospital-directory rows (OD-12).
func TestSearchOrdersByDepartmentThenName(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, [][4]any{
		{"新北", "甲醫院", "骨科", "王小明"},
		{"新北", "甲醫院", "牙科", "陳大文"},
		{"新北", "甲醫院", nil, "林一"},
		{"新北", "甲醫院", "骨科", "張小美"},
		{"新北", "甲醫院", "牙科", "李小華"},
	})

	tests := []struct {
		criteria search.Criteria
		want     []string
	}{
		{search.Criteria{SearchType: search.TypeHospital, SearchTerm: "甲醫院"}, []string{"李小華", "陳大文", "張小美", "王小明", "林一"}},
		{search.Criteria{SearchType: search.TypeAll, SearchTerm: "甲醫院"}, []string{"李小華", "陳大文", "張小美", "王小明", "林一"}},
		{search.Criteria{SearchType: search.TypeDepartment, SearchTerm: "科"}, []string{"李小華", "陳大文", "張小美", "王小明"}},
	}
	for _, tt := range tests {
		got, _, err := r.Search(tt.criteria, 0)
		require.NoError(t, err)
		require.Equal(t, tt.want, names(got), "search type %s", tt.criteria.SearchType)
	}
}

// 臺 and 台 match each other in every compared column, typed either way (OD-13).
func TestSearchMatchesBothTaiSpellings(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, [][4]any{
		{"台北", "臺北榮民總醫院", "內科", "王小明"},
		{"台北", "台北榮總", "心臟內科", "李小華"},
		{"台中", "測試醫院", "臺式推拿科", "張小美"},
		{"台中", "測試醫院", "內科", "張小臺"},
	})

	tests := []struct {
		input string
		want  []string
	}{
		{"臺北@醫院 臺北榮", []string{"王小明", "李小華"}},
		{"@醫院 台北榮", []string{"王小明", "李小華"}},
		{"@科別 臺式", []string{"張小美"}},
		{"張小台", []string{"張小臺"}},
	}
	for _, tt := range tests {
		got, _, err := r.Search(search.ParseCriteria(tt.input), 0)
		require.NoError(t, err)
		require.Equal(t, tt.want, names(got), "input %s", tt.input)
	}
}

// Every word must match; a word matches hospitals through its alias group and the
// 紀念/綜合 variants (OD-14). Rows are fictional apart from hospital names.
func TestSearchWordsAliasesAndHospitalVariants(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, [][4]any{
		{"台北", "三軍總醫院", "牙科", "王小明"},
		{"台北", "內湖三總", "牙科", "陳大文"},
		{"台北", "三總", "內科", "李小華"},
		{"新北", "亞東紀念醫院", "牙科", "張小美"},
		{"新北", "亞東醫院", "內科", "林志明"},
		{"台北", "甲牙科診所", "家醫科", "王大同"},
		{"高雄", "高雄醫學大學附設中和紀念醫院", "骨科", "林一"},
		{"高雄", "高醫岡山醫院", "骨科", "林二"},
	})

	tests := []struct {
		input string
		want  []string
	}{
		{"三總", []string{"李小華", "王小明", "陳大文"}},
		{"三總 牙科", []string{"王小明", "陳大文"}},
		{"牙科 三總", []string{"王小明", "陳大文"}},
		{"新北 三總", []string{}},
		{"@醫院 三軍總醫院", []string{"李小華", "王小明", "陳大文"}},
		// 王大同's hospital matches 牙科 (rank 1), the others only by department (rank 2).
		{"台北 牙科", []string{"王大同", "王小明", "陳大文"}},
		{"王 牙科", []string{"王大同", "王小明"}},
		{"亞東醫院", []string{"林志明", "張小美"}},
		{"亞東紀念醫院", []string{"林志明", "張小美"}},
		{"高醫", []string{"林一", "林二"}},
		{"高醫 岡山醫院", []string{"林二"}},
		// What the 同院同科醫師 buttons send: the full name finds only that hospital,
		// and a blog spelling finds the whole alias group.
		{"高雄 高雄醫學大學附設中和紀念醫院 骨科", []string{"林一"}},
		{"台北 內湖三總 牙科", []string{"王小明", "陳大文"}},
	}
	for _, tt := range tests {
		got, _, err := r.Search(search.ParseCriteria(tt.input), 0)
		require.NoError(t, err)
		require.Equal(t, tt.want, names(got), "input %s", tt.input)
	}

	// A row is ranked by the first field any word matches: 王 matches the names.
	_, stats, err := r.Search(search.ParseCriteria("王 牙科"), 0)
	require.NoError(t, err)
	require.Equal(t, MatchCounts{Name: 2}, stats.MatchCounts)
	_, stats, err = r.Search(search.ParseCriteria("三總 牙科"), 0)
	require.NoError(t, err)
	require.Equal(t, MatchCounts{Hospital: 2}, stats.MatchCounts)
}

func TestDirectoryHospitals(t *testing.T) {
	r := openIntegrationDB(t)
	r.db.MustExec(`INSERT INTO medical_personnel (city, hospital, department, name, source) VALUES
		('台北', '甲醫院', '內科', '王小明', 'hospital_directory'),
		('新北', '乙醫院', '內科', '李小華', 'hospital_directory'),
		('新北', '乙醫院', '牙科', '張小美', 'hospital_directory'),
		('新北', '乙醫院', '牙科', '陳大文', 'blog'),
		('台北', '丙醫院', '內科', '林一', 'blog'),
		('台北', '丁醫院', '內科', '林二', NULL)`)

	got, err := r.DirectoryHospitals(context.Background())
	require.NoError(t, err)
	require.Equal(t, []DirectoryHospital{
		{City: "新北", Hospital: "乙醫院", Count: 2},
		{City: "台北", Hospital: "甲醫院", Count: 1},
	}, got)
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

	require.NoError(t, r.LogSearch(ctx, search.Criteria{SearchType: search.TypeAll, SearchTerm: "林", City: &taipei}, 4, ChannelLINE))
	require.NoError(t, r.LogSearch(ctx, search.Criteria{SearchType: search.TypeHospital, SearchTerm: "三總"}, 0, ChannelWeb))

	type logRow struct {
		SearchType  string         `db:"search_type"`
		City        sql.NullString `db:"city"`
		SearchTerm  string         `db:"search_term"`
		ResultCount int            `db:"result_count"`
		Channel     string         `db:"channel"`
		HasTS       bool           `db:"has_ts"`
	}
	var got []logRow
	require.NoError(t, r.db.Select(&got,
		"SELECT search_type, city, search_term, result_count, channel, created_at IS NOT NULL AS has_ts FROM search_log ORDER BY id"))
	require.Equal(t, []logRow{
		{SearchType: "all", City: sql.NullString{String: "台北", Valid: true}, SearchTerm: "林", ResultCount: 4, Channel: "line", HasTS: true},
		{SearchType: "hospital", City: sql.NullString{}, SearchTerm: "三總", ResultCount: 0, Channel: "web", HasTS: true},
	}, got)
}

func TestSearchLogHasNoUserColumn(t *testing.T) {
	r := openIntegrationDB(t)

	var cols []string
	require.NoError(t, r.db.Select(&cols,
		"SELECT column_name FROM information_schema.columns WHERE table_name = 'search_log' ORDER BY ordinal_position"))
	// channel says LINE or web (OD-17), not who searched.
	require.Equal(t, []string{"id", "created_at", "search_type", "city", "search_term", "result_count", "channel"}, cols)
}

func TestSearchLogRejectsUnknownType(t *testing.T) {
	r := openIntegrationDB(t)

	_, err := r.db.Exec("INSERT INTO search_log (search_type, search_term, result_count) VALUES ('name', 'x', 0)")
	require.Error(t, err)
	_, err = r.db.Exec("INSERT INTO search_log (search_type, search_term, result_count, channel) VALUES ('all', 'x', 0, 'app')")
	require.Error(t, err, "channel is line or web")
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
