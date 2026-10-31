// Package crawl fetches doctor profile pages from hospital websites and plans
// how to sync them into medical_personnel (docs/open-decisions.md OD-7, OD-10).
package crawl

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log/slog"
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
	// lineBreakRe splits free-text fields into lines.
	lineBreakRe = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</li>|</div>`)
)

// htmlText strips tags, unescapes entities and trims the result.
func htmlText(raw string) string {
	return strings.TrimSpace(html.UnescapeString(tagRe.ReplaceAllString(raw, "")))
}

// htmlLines splits a free-text HTML field at line breaks (see lineBreakRe) into
// its trimmed, non-empty lines.
func htmlLines(raw string) []string {
	var lines []string
	for _, line := range strings.Split(htmlText(lineBreakRe.ReplaceAllString(raw, "\n")), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			lines = append(lines, s)
		}
	}
	return lines
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

// Some sites list doctors whose profile page is broken or unpublished: 長庚
// returns 500 for 6.5% of them on every attempt, 高醫 redirects some to its home
// page. A source that skips such profiles (profileUnavailable) counts them in a
// profileSkips: once minProfilesForSkipCheck profiles have been requested, more
// than maxSkipPercent of them skipped means the site itself is broken, and the
// crawl fails then instead of spending the retries on every remaining profile.
const (
	maxSkipPercent          = 15
	minProfilesForSkipCheck = 100
)

// profileUnavailable reports whether a profile fetch failed in a way a skipping
// source tolerates: every retry failed (errUnavailable) or the page redirected.
func profileUnavailable(err error) bool {
	return errors.Is(err, errUnavailable) || errors.Is(err, errRedirected)
}

type profileSkips struct {
	requested, skipped int
}

// add counts one profile request and returns an error once too many are skipped.
func (p *profileSkips) add(skipped bool) error {
	p.requested++
	if skipped {
		p.skipped++
	}
	if p.requested >= minProfilesForSkipCheck && p.skipped*100 > p.requested*maxSkipPercent {
		return fmt.Errorf("%d of %d profiles unavailable", p.skipped, p.requested)
	}
	return nil
}

// log records the counts only, never which doctors were skipped.
func (p *profileSkips) log(source string) {
	slog.Info("skipped unavailable profiles", "source", source, "skipped", p.skipped, "requested", p.requested)
}
