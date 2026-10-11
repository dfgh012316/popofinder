package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dfgh012316/popofinder/internal/crawl"
	"github.com/jmoiron/sqlx"
)

// LoadCrawlState returns the rows a crawl source wrote before and the source
// keys that were taken down (docs/open-decisions.md OD-10).
func (r *Repository) LoadCrawlState(ctx context.Context, source string) ([]crawl.ExistingRow, map[string]bool, error) {
	pattern := source + ":%"
	var rows []crawl.ExistingRow
	if err := r.db.SelectContext(ctx, &rows,
		"SELECT id, source_key, name, city, hospital, department, education, source_url FROM medical_personnel WHERE source_key LIKE $1 ORDER BY id",
		pattern,
	); err != nil {
		return nil, nil, fmt.Errorf("repository: load crawl state: %w", err)
	}
	var keys []string
	if err := r.db.SelectContext(ctx, &keys, "SELECT source_key FROM takedown_suppressions WHERE source_key LIKE $1", pattern); err != nil {
		return nil, nil, fmt.Errorf("repository: load crawl state: %w", err)
	}
	suppressed := make(map[string]bool, len(keys))
	for _, k := range keys {
		suppressed[k] = true
	}
	return rows, suppressed, nil
}

// ApplyCrawlPlan writes plan in one transaction. History rows get reason
// "crawl:<source>"; source_checked_at is checkedOn's calendar date.
func (r *Repository) ApplyCrawlPlan(ctx context.Context, source string, plan crawl.SyncPlan, checkedOn time.Time) error {
	// Pass the DATE as text so the stored day is always checkedOn's own calendar
	// date, independent of how the driver would encode a time.Time.
	date := checkedOn.Format("2006-01-02")
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("repository: apply crawl plan: %w", err)
	}
	if err := applyCrawlPlan(ctx, tx, source, plan, date); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("repository: apply crawl plan: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("repository: apply crawl plan: %w", err)
	}
	return nil
}

func applyCrawlPlan(ctx context.Context, tx *sqlx.Tx, source string, plan crawl.SyncPlan, date string) error {
	if _, err := tx.ExecContext(ctx, "SELECT set_config('popo.change_reason', $1, true)", "crawl:"+source); err != nil {
		return err
	}
	for _, row := range plan.Inserts {
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO medical_personnel (city, hospital, department, name, education, source, source_url, source_key, source_checked_at) VALUES ($1, $2, $3, $4, $5, 'hospital_directory', $6, $7, $8)",
			row.City, row.Hospital, nullIfEmpty(row.Department), row.Name, row.Education, row.URL, row.Key, date,
		); err != nil {
			return err
		}
	}
	for _, u := range plan.Updates {
		if _, err := tx.ExecContext(ctx,
			"UPDATE medical_personnel SET name = $1, city = $2, hospital = $3, department = $4, education = $5, source_url = $6, source_checked_at = $7, updated_at = now() WHERE id = $8",
			u.Row.Name, u.Row.City, u.Row.Hospital, nullIfEmpty(u.Row.Department), u.Row.Education, u.Row.URL, date, u.ID,
		); err != nil {
			return err
		}
	}
	for _, id := range plan.TouchIDs {
		if _, err := tx.ExecContext(ctx, "UPDATE medical_personnel SET source_checked_at = $1, updated_at = now() WHERE id = $2", date, id); err != nil {
			return err
		}
	}
	for _, id := range plan.DeleteIDs {
		if _, err := tx.ExecContext(ctx, "DELETE FROM medical_personnel WHERE id = $1", id); err != nil {
			return err
		}
	}
	return nil
}

func nullIfEmpty(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
