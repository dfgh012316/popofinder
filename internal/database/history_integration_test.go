//go:build integration

package database

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
)

type historyRow struct {
	PersonnelID int            `db:"personnel_id"`
	Op          string         `db:"op"`
	OldHospital sql.NullString `db:"old_hospital"`
	NewHospital sql.NullString `db:"new_hospital"`
	Reason      sql.NullString `db:"reason"`
}

// historyRows returns medical_personnel_history in insertion order, projecting
// the hospital field of old_row/new_row so tests can compare before and after.
func historyRows(t *testing.T, r *Repository) []historyRow {
	t.Helper()
	var rows []historyRow
	require.NoError(t, r.db.Select(&rows,
		"SELECT personnel_id, op, old_row->>'hospital' AS old_hospital, new_row->>'hospital' AS new_hospital, reason FROM medical_personnel_history ORDER BY id"))
	return rows
}

func valid(s string) sql.NullString {
	return sql.NullString{String: s, Valid: true}
}

func TestHistoryRecordsInsert(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, [][4]any{{"台北", "甲醫院", "內科", "王一"}})

	require.Equal(t, []historyRow{
		{PersonnelID: 1, Op: "INSERT", NewHospital: valid("甲醫院")},
	}, historyRows(t, r))
}

func TestHistoryRecordsUpdate(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, [][4]any{{"台北", "甲醫院", "內科", "王一"}})
	r.db.MustExec("UPDATE medical_personnel SET hospital = '乙醫院' WHERE id = 1")

	got := historyRows(t, r)
	require.Len(t, got, 2)
	require.Equal(t, historyRow{PersonnelID: 1, Op: "UPDATE", OldHospital: valid("甲醫院"), NewHospital: valid("乙醫院")}, got[1])
}

func TestHistorySkipsNoopUpdate(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, [][4]any{{"台北", "甲醫院", "內科", "王一"}})
	r.db.MustExec("UPDATE medical_personnel SET hospital = hospital WHERE id = 1")

	got := historyRows(t, r)
	require.Len(t, got, 1)
	require.Equal(t, "INSERT", got[0].Op)
}

func TestHistoryRecordsDelete(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, [][4]any{{"台北", "甲醫院", "內科", "王一"}})
	r.db.MustExec("DELETE FROM medical_personnel WHERE id = 1")

	got := historyRows(t, r)
	require.Len(t, got, 2)
	require.Equal(t, historyRow{PersonnelID: 1, Op: "DELETE", OldHospital: valid("甲醫院")}, got[1])
}

func TestHistoryReasonFromSessionSetting(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, [][4]any{{"台北", "甲醫院", "內科", "王一"}})
	ctx := context.Background()

	conn, err := r.db.Connx(ctx)
	require.NoError(t, err)
	defer conn.Close()

	tx, err := conn.BeginTxx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.Exec("SET LOCAL popo.change_reason = '資料清理'")
	require.NoError(t, err)
	_, err = tx.Exec("UPDATE medical_personnel SET hospital = '乙醫院' WHERE id = 1")
	require.NoError(t, err)
	require.NoError(t, tx.Commit())

	// Same connection, outside the transaction: the placeholder now reads as '' and must be stored as NULL.
	_, err = conn.ExecContext(ctx, "UPDATE medical_personnel SET hospital = '丙醫院' WHERE id = 1")
	require.NoError(t, err)

	var reasons []sql.NullString
	for _, h := range historyRows(t, r) {
		if h.Op == "UPDATE" {
			reasons = append(reasons, h.Reason)
		}
	}
	require.Equal(t, []sql.NullString{valid("資料清理"), {}}, reasons)
}

func TestHistoryAllowsTakedownPurge(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, [][4]any{
		{"台北", "甲醫院", "內科", "王一"},
		{"台北", "甲醫院", "內科", "王二"},
	})
	r.db.MustExec("UPDATE medical_personnel SET hospital = '乙醫院' WHERE id = 1")

	// OD-8/OD-9 takedown: takedown_personnel deletes the row, purges its history and keeps one content-free DELETE record.
	r.db.MustExec("SELECT takedown_personnel(1)")

	var leaked int
	require.NoError(t, r.db.Get(&leaked,
		"SELECT count(*) FROM medical_personnel_history WHERE personnel_id = 1 AND (old_row IS NOT NULL OR new_row IS NOT NULL)"))
	require.Zero(t, leaked)
	var remaining int
	require.NoError(t, r.db.Get(&remaining, "SELECT count(*) FROM medical_personnel WHERE id = 1"))
	require.Zero(t, remaining)
	require.Equal(t, []historyRow{
		{PersonnelID: 2, Op: "INSERT", NewHospital: valid("甲醫院")},
		{PersonnelID: 1, Op: "DELETE", Reason: valid("takedown")},
	}, historyRows(t, r))
}

func TestTakedownPersonnelRejectsUnknownID(t *testing.T) {
	r := openIntegrationDB(t)
	seedPersonnel(t, r, [][4]any{{"台北", "甲醫院", "內科", "王一"}})

	_, err := r.db.Exec("SELECT takedown_personnel(999)")
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")

	require.Equal(t, []historyRow{
		{PersonnelID: 1, Op: "INSERT", NewHospital: valid("甲醫院")},
	}, historyRows(t, r))
	var count int
	require.NoError(t, r.db.Get(&count, "SELECT count(*) FROM medical_personnel"))
	require.Equal(t, 1, count)
}
