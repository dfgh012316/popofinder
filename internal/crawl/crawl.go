// Package crawl fetches doctor profile pages from hospital websites and plans
// how to sync them into medical_personnel (docs/open-decisions.md OD-7, OD-10).
package crawl

import (
	"context"
	"html"
	"regexp"
	"strings"
)

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

var (
	tagRe      = regexp.MustCompile(`<[^>]*>`)
	listItemRe = regexp.MustCompile(`(?s)<li>(.*?)</li>`)
)

// htmlText strips tags, unescapes entities and trims the result.
func htmlText(raw string) string {
	return strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(raw, "")))
}

// doctorIndex collects doctor IDs from department pages: each ID once, in the
// order first seen, with the code of the first department listing it (the
// profile is fetched under that department) and every department name it is
// listed under, in department order.
type doctorIndex struct {
	order []string
	first map[string]string
	depts map[string][]string
}

func newDoctorIndex() *doctorIndex {
	return &doctorIndex{first: map[string]string{}, depts: map[string][]string{}}
}

func (x *doctorIndex) add(id, deptCode, deptName string) {
	if _, ok := x.first[id]; !ok {
		x.first[id] = deptCode
		x.order = append(x.order, id)
	}
	x.depts[id] = append(x.depts[id], deptName)
}
