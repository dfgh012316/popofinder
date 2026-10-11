// Package crawl fetches doctor profile pages from hospital websites and plans
// how to sync them into medical_personnel (docs/open-decisions.md OD-7, OD-10).
package crawl

import "context"

// Doctor is one doctor as published on a hospital website.
type Doctor struct {
	Key         string // "<source>:<site doctor id>", e.g. "femh:42522"
	Name        string
	City        string
	Hospital    string
	Departments []string // in site order; BuildSyncPlan dedupes and joins them
	Education   []string // one entry per line of the site's 學歷 field
	URL         string   // profile page
}

// Source crawls one hospital website. Crawl returns an error if any page fails,
// so callers never sync a partial crawl.
type Source interface {
	Name() string
	Crawl(ctx context.Context) ([]Doctor, error)
}
