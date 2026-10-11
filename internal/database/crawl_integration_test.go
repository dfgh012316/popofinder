//go:build integration

package database

import (
	"context"
	"testing"
	"time"

	"github.com/dfgh012316/popofinder/internal/crawl"
	"github.com/stretchr/testify/require"
)

var taipei = time.FixedZone("Asia/Taipei", 8*60*60)

// crawlDoctor is a fictional crawled doctor with one education line.
func crawlDoctor(key, education string) crawl.Doctor {
	return crawl.Doctor{
		Key:         key,
		Name:        "王一",
		City:        "新北",
		Hospital:    "亞東紀念醫院",
		Departments: []string{"內科"},
		Education:   []string{education},
		URL:         "https://example.invalid/" + key,
	}
}

// syncFemh runs one crawl round the way cmd/crawl does: load state, plan, apply.
func syncFemh(t *testing.T, r *Repository, crawled []crawl.Doctor, checkedOn time.Time) crawl.SyncPlan {
	t.Helper()
	ctx := context.Background()
	existing, suppressed, err := r.LoadCrawlState(ctx, "femh")
	require.NoError(t, err)
	plan := crawl.BuildSyncPlan(crawled, existing, suppressed)
	require.NoError(t, r.ApplyCrawlPlan(ctx, "femh", plan, checkedOn))
	return plan
}

func idOf(t *testing.T, r *Repository, key string) int {
	t.Helper()
	var id int
	require.NoError(t, r.db.Get(&id, "SELECT id FROM medical_personnel WHERE source_key = $1", key))
	return id
}

func TestApplyCrawlPlanRoundTrip(t *testing.T) {
	r := openIntegrationDB(t)
	const ntu, ym = "國立台灣大學醫學系", "國立陽明大學醫學系"

	// Round 1 at 00:30 Taipei time, which is still the previous day in UTC.
	syncFemh(t, r, []crawl.Doctor{
		crawlDoctor("femh:1", ntu),
		crawlDoctor("femh:2", ntu),
		crawlDoctor("femh:4", ntu),
	}, time.Date(2026, 10, 12, 0, 30, 0, 0, taipei))

	var rows []struct {
		Source  string `db:"source"`
		Status  string `db:"verification_status"`
		Checked string `db:"checked"`
	}
	require.NoError(t, r.db.Select(&rows, "SELECT source, verification_status, source_checked_at::text AS checked FROM medical_personnel ORDER BY id"))
	require.Len(t, rows, 3)
	for _, row := range rows {
		require.Equal(t, "hospital_directory", row.Source)
		require.Equal(t, "unverified", row.Status)
		require.Equal(t, "2026-10-12", row.Checked)
	}
	hist := historyRows(t, r)
	require.Len(t, hist, 3)
	for _, h := range hist {
		require.Equal(t, "INSERT", h.Op)
		require.Equal(t, valid("crawl:femh"), h.Reason)
	}

	a, b, d := idOf(t, r, "femh:1"), idOf(t, r, "femh:2"), idOf(t, r, "femh:4")

	// Round 2: A unchanged, B's education changed, C new, D gone.
	syncFemh(t, r, []crawl.Doctor{
		crawlDoctor("femh:1", ntu),
		crawlDoctor("femh:2", ym),
		crawlDoctor("femh:3", ntu),
	}, time.Date(2026, 11, 12, 0, 30, 0, 0, taipei))

	var checked string
	require.NoError(t, r.db.Get(&checked, "SELECT source_checked_at::text FROM medical_personnel WHERE id = $1", a))
	require.Equal(t, "2026-11-12", checked)
	var count int
	require.NoError(t, r.db.Get(&count, "SELECT count(*) FROM medical_personnel_history WHERE personnel_id = $1", a))
	require.Equal(t, 1, count, "a checked-date-only update must not add history")

	var education string
	require.NoError(t, r.db.Get(&education, "SELECT education FROM medical_personnel WHERE id = $1", b))
	require.Equal(t, ym, education)
	require.NoError(t, r.db.Get(&count, "SELECT count(*) FROM medical_personnel_history WHERE personnel_id = $1 AND op = 'UPDATE' AND reason = 'crawl:femh'", b))
	require.Equal(t, 1, count)

	require.NoError(t, r.db.Get(&count, "SELECT count(*) FROM medical_personnel WHERE source_key = 'femh:3'"))
	require.Equal(t, 1, count)

	require.NoError(t, r.db.Get(&count, "SELECT count(*) FROM medical_personnel WHERE id = $1", d))
	require.Zero(t, count)
	require.NoError(t, r.db.Get(&count, "SELECT count(*) FROM medical_personnel_history WHERE personnel_id = $1 AND op = 'DELETE' AND reason = 'crawl:femh' AND old_row IS NOT NULL", d))
	require.Equal(t, 1, count)
}

func TestLoadCrawlStateScopesBySource(t *testing.T) {
	r := openIntegrationDB(t)
	r.db.MustExec("INSERT INTO medical_personnel (city, hospital, name) VALUES ('台北', '甲醫院', '王一')")
	r.db.MustExec("INSERT INTO medical_personnel (city, hospital, name, source_key) VALUES ('桃園', '乙醫院', '王二', 'cgmh:1')")
	r.db.MustExec("INSERT INTO medical_personnel (city, hospital, name, source_key) VALUES ('新北', '亞東紀念醫院', '王三', 'femh:1')")
	r.db.MustExec("INSERT INTO takedown_suppressions (source_key) VALUES ('femh:9'), ('cgmh:9')")

	rows, suppressed, err := r.LoadCrawlState(context.Background(), "femh")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "femh:1", rows[0].Key)
	require.Equal(t, map[string]bool{"femh:9": true}, suppressed)
}

func TestCrawlDoesNotReAddTakenDownDoctor(t *testing.T) {
	r := openIntegrationDB(t)
	a := crawlDoctor("femh:1", "國立台灣大學醫學系")
	syncFemh(t, r, []crawl.Doctor{a}, time.Date(2026, 10, 12, 0, 30, 0, 0, taipei))
	r.db.MustExec("SELECT takedown_personnel($1)", idOf(t, r, "femh:1"))

	plan := syncFemh(t, r, []crawl.Doctor{a}, time.Date(2026, 11, 12, 0, 30, 0, 0, taipei))

	require.Equal(t, 1, plan.SkippedSuppressed)
	require.Empty(t, plan.Inserts)
	var count int
	require.NoError(t, r.db.Get(&count, "SELECT count(*) FROM medical_personnel"))
	require.Zero(t, count)
}
