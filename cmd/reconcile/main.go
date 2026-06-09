package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/reconcile"
)

func main() {
	dryRun := flag.Bool("dry-run", false, "只印出計畫,不寫入資料庫")
	baseURL := flag.String("base-url", "https://popolist999.blogspot.com", "blog base URL")
	flag.Parse()

	ctx := context.Background()

	postHTML, postURL, err := reconcile.FetchPlaintextPost(ctx, *baseURL)
	if err != nil {
		slog.Error("fetch post", "err", err)
		os.Exit(1)
	}

	records := reconcile.ParsePost(postHTML)
	slog.Info("parsed", "count", len(records))

	db, err := database.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("connect database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	var existing []reconcile.ExistingRow
	if err := db.Select(&existing, "SELECT id, name, hospital, (source IS NOT NULL) AS has_source FROM medical_personnel"); err != nil {
		slog.Error("load existing rows", "err", err)
		os.Exit(1)
	}

	plan := reconcile.BuildPlan(records, existing)
	slog.Info("plan", "mark_blog", len(plan.MarkBlogIDs), "inserts", len(plan.Inserts))

	if *dryRun {
		slog.Info("dry-run, no writes")
		return
	}

	tx, err := db.Beginx()
	if err != nil {
		slog.Error("begin tx", "err", err)
		os.Exit(1)
	}

	for _, id := range plan.MarkBlogIDs {
		if _, err := tx.Exec("UPDATE medical_personnel SET source='blog', source_url=$1, updated_at=now() WHERE id=$2", postURL, id); err != nil {
			_ = tx.Rollback()
			slog.Error("update row", "id", id, "err", err)
			os.Exit(1)
		}
	}

	for _, r := range plan.Inserts {
		var dept any
		if r.Department != "" {
			dept = r.Department
		}
		if _, err := tx.Exec(
			"INSERT INTO medical_personnel (city, hospital, department, name, education, source, verification_status, source_url) VALUES ($1,$2,$3,$4,$5,'blog','unverified',$6)",
			r.City, r.Hospital, dept, r.Name, r.Education, postURL,
		); err != nil {
			_ = tx.Rollback()
			slog.Error("insert row", "name", r.Name, "err", err)
			os.Exit(1)
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("commit tx", "err", err)
		os.Exit(1)
	}

	slog.Info("done", "marked", len(plan.MarkBlogIDs), "inserted", len(plan.Inserts))
}
