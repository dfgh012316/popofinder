package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/dfgh012316/popofinder/internal/crawl"
	"github.com/dfgh012316/popofinder/internal/database"
)

func main() {
	sources := flag.String("sources", "", "要爬的來源,逗號分隔(目前支援:femh,shh,mmh,cgh,cgmh,kmuh,kmsh,smile)")
	dryRun := flag.Bool("dry-run", false, "只印出計畫,不寫入資料庫")
	flag.Parse()

	fetcher := crawl.NewFetcher(time.Second)
	registry := map[string]crawl.Source{
		"femh":  crawl.NewFemh("https://www.femh.org.tw", fetcher),
		"shh":   crawl.NewShh("https://shh.tmu.edu.tw", fetcher),
		"mmh":   crawl.NewMmh("https://www.mmh.org.tw", fetcher),
		"cgh":   crawl.NewCgh("https://www.cgh.org.tw", "https://sijhih.cgh.org.tw", "https://hsinchu.cgh.org.tw", "https://neihu.cgh.org.tw", fetcher),
		"cgmh":  crawl.NewCgmh("https://www.cgmh.org.tw", fetcher),
		"kmuh":  crawl.NewKmuh("https://www.kmuh.org.tw", "https://www.kmugh.org.tw", fetcher),
		"kmsh":  crawl.NewKmsh("https://www.kmsh.org.tw/Web/wwwKMHK/dept/8300/2024KMSH/pro", fetcher),
		"smile": crawl.NewSmile("https://www.smile-dental.tw", fetcher),
	}

	// Validate before connecting so a typo never touches the database.
	var selected []crawl.Source
	for _, name := range strings.Split(*sources, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			slog.Error("no sources")
			os.Exit(2)
		}
		s, ok := registry[name]
		if !ok {
			slog.Error(fmt.Sprintf("unknown source %q", name))
			os.Exit(2)
		}
		selected = append(selected, s)
	}

	db, err := database.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		slog.Error("connect database", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	repo := database.NewRepository(db)

	// distroless has no tzdata, so time.LoadLocation("Asia/Taipei") would fail there.
	checkedOn := time.Now().In(time.FixedZone("Asia/Taipei", 8*60*60))
	ctx := context.Background()

	// No log carries names or education (OD-10): the plan log is counts only, and a
	// crawl error names the failing page URL (site IDs only) so the page can be found.
	failed := false
	for _, s := range selected {
		name := s.Name()
		doctors, err := s.Crawl(ctx)
		if err != nil {
			slog.Error("crawl", "source", name, "err", err)
			failed = true
			continue
		}
		existing, suppressed, err := repo.LoadCrawlState(ctx, name)
		if err != nil {
			slog.Error("load crawl state", "source", name, "err", err)
			failed = true
			continue
		}
		plan := crawl.BuildSyncPlan(doctors, existing, suppressed)
		slog.Info("plan", "source", name,
			"crawled", len(doctors),
			"insert", len(plan.Inserts),
			"update", len(plan.Updates),
			"touch", len(plan.TouchIDs),
			"delete", len(plan.DeleteIDs),
			"skipped_suppressed", plan.SkippedSuppressed,
			"skipped_no_degree", plan.SkippedNoDegree,
			"delete_blocked", plan.DeleteBlocked,
		)
		if plan.DeleteBlocked {
			slog.Error("delete blocked", "source", name)
			failed = true
		}
		if *dryRun {
			continue
		}
		if err := repo.ApplyCrawlPlan(ctx, name, plan, checkedOn); err != nil {
			slog.Error("apply crawl plan", "source", name, "err", err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}
