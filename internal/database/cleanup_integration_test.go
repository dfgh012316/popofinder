//go:build integration

package database

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// runBlogCleanup runs the one-off cleanup script in one transaction, as psql -1 does.
// pgx sends a statement without arguments over the simple protocol, so the whole
// script goes in one Exec.
func runBlogCleanup(t *testing.T, r *Repository) {
	t.Helper()
	script, err := os.ReadFile("../../scripts/cleanup/2026-10-12-blog-cleanup.sql")
	require.NoError(t, err)
	tx := r.db.MustBegin()
	_, err = tx.Exec(string(script))
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
}

// Fictional rows; ids 1–13 follow the insert order (OD-13).
func TestBlogCleanupScript(t *testing.T) {
	r := openIntegrationDB(t)
	r.db.MustExec(`INSERT INTO medical_personnel (city, hospital, department, name, education, source) VALUES
		('台北', '在學中', NULL, '王小明', '波蘭甲醫學大學', 'blog'),
		('台北', '待實習', '牙科', '李小華', '西班牙乙大學', NULL),
		('台北', '丙診所/丁診所', '牙科', '張小美', '西班牙乙大學牙醫系', 'blog'),
		('台北', '丙診所丁診所', '牙科', '張小美', '西班牙乙大學牙醫系', NULL),
		('台北', '丙診所丁診所', '牙科', '張小美', '捷克丙大學', NULL),
		('新北', '戊診所', '牙醫', '陳大文', '大學', 'blog'),
		('新北', '戊 診所', '牙科', '陳大文', '波蘭甲醫學大學', 'blog'),
		('新北', '戊診所.', '牙醫', '陳大文', '西班牙乙大學牙醫系', 'blog'),
		('新北', '戊診所', '牙醫', '陳大文', '波蘭甲醫學大學', 'blog'),
		('新北', '戊診所', '牙醫', '林一', '大學', 'blog'),
		('新北', '戊診所', '牙醫', '陳大文', '國立甲大學醫學系', 'hospital_directory'),
		('台北', '甲實習醫院', '內科', '王小明', '國立甲大學醫學系', 'hospital_directory'),
		('台北', '戊診所', '牙醫', '陳大文', '大學', 'blog')`)

	type cleanedRow struct {
		ID         int    `db:"id"`
		Hospital   string `db:"hospital"`
		Department string `db:"department"`
		Name       string `db:"name"`
		Education  string `db:"education"`
		Source     string `db:"source"`
	}
	readRows := func() []cleanedRow {
		var rows []cleanedRow
		require.NoError(t, r.db.Select(&rows, `SELECT id, hospital, coalesce(department, '') AS department, name,
			coalesce(education, '') AS education, coalesce(source, '') AS source
			FROM medical_personnel ORDER BY id`))
		return rows
	}

	type reasonCount struct {
		Reason string `db:"reason"`
		Op     string `db:"op"`
		N      int    `db:"n"`
	}
	readReasons := func() []reasonCount {
		var counts []reasonCount
		require.NoError(t, r.db.Select(&counts, `SELECT reason, op, count(*) AS n FROM medical_personnel_history
			WHERE reason LIKE 'cleanup:%' GROUP BY reason, op ORDER BY reason, op`))
		return counts
	}

	wantRows := []cleanedRow{
		{3, "丙診所/丁診所", "牙科", "張小美", "西班牙乙大學牙醫系", "blog"},
		{5, "丙診所丁診所", "牙科", "張小美", "捷克丙大學", ""},
		{6, "戊診所", "牙醫、牙科", "陳大文", "波蘭甲醫學大學；西班牙乙大學牙醫系", "blog"},
		{10, "戊診所", "牙醫", "林一", "大學", "blog"},
		{11, "戊診所", "牙醫", "陳大文", "國立甲大學醫學系", "hospital_directory"},
		{12, "甲實習醫院", "內科", "王小明", "國立甲大學醫學系", "hospital_directory"},
		{13, "戊診所", "牙醫", "陳大文", "大學", "blog"},
	}
	wantReasons := []reasonCount{
		{"cleanup:blog-merge", "DELETE", 3},
		{"cleanup:blog-merge", "UPDATE", 1},
		{"cleanup:non-practitioner", "DELETE", 2},
		{"cleanup:unclassified-duplicate", "DELETE", 1},
	}

	runBlogCleanup(t, r)
	require.Equal(t, wantRows, readRows())
	require.Equal(t, wantReasons, readReasons())

	// A second run changes nothing.
	runBlogCleanup(t, r)
	require.Equal(t, wantRows, readRows())
	require.Equal(t, wantReasons, readReasons())
}
