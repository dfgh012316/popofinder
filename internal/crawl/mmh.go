package crawl

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const (
	mmhHospital = "馬偕紀念醫院"
	mmhCity     = "台北"
)

var (
	mmhListItemRe  = regexp.MustCompile(`(?s)<li class='list'>\s*<span class='name'><div><a href='doctor_view\.php\?depid=(\w*)(?:&amp;|&)did=(\d*)' target='_new'>(.*?)</a></div>\s*<div>(.*?)<sub>(.*?)</sub></div></span>`)
	mmhCountRe     = regexp.MustCompile(`<h6>共 (\d+) 名</h6>`)
	mmhAreaRe      = regexp.MustCompile(`DMZWEBEMRA651/Default\.aspx\?a=(TP|TS)`)
	mmhNameRe      = regexp.MustCompile(`(?s)<h2 class="event-hed-title">(.*?)</h2>`)
	mmhRoleRe      = regexp.MustCompile(`^\s*[(（][^)）]*[)）]`)
	mmhEducationRe = regexp.MustCompile(`(?s)<h2 class="event-ch-title">主要學歷</h2>\s*<ul class="info-list">(.*?)</ul>\s*<h2 class="event-ch-title">`)
	mmhBreakRe     = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</li>|</div>`)
)

// Mmh crawls 馬偕紀念醫院 (www.mmh.org.tw): the 台北 doctor list → profile pages.
// Without a session cookie the list shows only 台北 (doctors seeing patients only
// at 淡水 are not on it).
type Mmh struct {
	baseURL string
	fetcher *Fetcher
}

func NewMmh(baseURL string, f *Fetcher) *Mmh {
	return &Mmh{baseURL: baseURL, fetcher: f}
}

func (s *Mmh) Name() string { return "mmh" }

type mmhEntry struct {
	DepID, DID, Dept string
}

func (s *Mmh) listURL() string { return s.baseURL + "/find_doctor_list.php" }

func (s *Mmh) profileURL(depID, did string) string {
	return s.baseURL + "/doctor_view.php?depid=" + depID + "&did=" + did
}

// parseMmhList returns the list entries that have a profile, in page order and
// deduped by did. It fails unless the list is the 台北 one and its "共 N 名"
// equals the number of entries parsed, so a changed layout cannot silently drop doctors.
func parseMmhList(page string) ([]mmhEntry, error) {
	if m := mmhAreaRe.FindStringSubmatch(page); m == nil || m[1] != "TP" {
		return nil, fmt.Errorf("crawl: mmh list is not the 台北 list")
	}
	matches := mmhListItemRe.FindAllStringSubmatch(page, -1)
	count := ""
	if m := mmhCountRe.FindStringSubmatch(page); m != nil {
		count = m[1]
	}
	if count != fmt.Sprint(len(matches)) {
		return nil, fmt.Errorf("crawl: mmh list says %s doctors, parsed %d", count, len(matches))
	}
	var entries []mmhEntry
	seen := map[string]bool{}
	for _, m := range matches {
		if m[2] == "" || seen[m[2]] {
			continue
		}
		seen[m[2]] = true
		entries = append(entries, mmhEntry{DepID: m[1], DID: m[2], Dept: htmlText(m[5])})
	}
	return entries, nil
}

// parseMmhProfile reads the name from the event-hed-title <h2> (<title> is the
// hospital name) and the lines of the 主要學歷 block, which runs to the next
// section <h2> because it may contain a nested </ul>; a profile without a 主要學歷
// block has nil education.
func parseMmhProfile(page string) (name string, education []string, err error) {
	if m := mmhNameRe.FindStringSubmatch(page); m != nil {
		// Some headings start with a role, "(顧問)姓名", or add an English name or degrees:
		// "姓名(English Name)", "姓名,English Name,M.D.".
		n := mmhRoleRe.ReplaceAllString(htmlText(m[1]), "")
		if i := strings.IndexAny(n, "(（,"); i >= 0 {
			n = n[:i]
		}
		name = strings.Join(strings.Fields(n), "")
	}
	if name == "" {
		return "", nil, fmt.Errorf("crawl: mmh profile without name")
	}
	if m := mmhEducationRe.FindStringSubmatch(page); m != nil {
		for _, line := range strings.Split(htmlText(mmhBreakRe.ReplaceAllString(m[1], "\n")), "\n") {
			if s := strings.TrimSpace(line); s != "" {
				education = append(education, s)
			}
		}
	}
	return name, education, nil
}

// Crawl returns every doctor on the 台北 list that has a profile. Any failed page
// fails the whole crawl.
func (s *Mmh) Crawl(ctx context.Context) ([]Doctor, error) {
	list, err := s.fetcher.Get(ctx, s.listURL())
	if err != nil {
		return nil, fmt.Errorf("crawl: mmh list: %w", err)
	}
	entries, err := parseMmhList(list)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("crawl: mmh list has no doctors")
	}

	doctors := make([]Doctor, 0, len(entries))
	for _, e := range entries {
		u := s.profileURL(e.DepID, e.DID)
		page, err := s.fetcher.Get(ctx, u)
		if err != nil {
			return nil, fmt.Errorf("crawl: mmh doctor %s: %w", e.DID, err)
		}
		name, education, err := parseMmhProfile(page)
		if err != nil {
			return nil, fmt.Errorf("crawl: mmh doctor %s: %w", e.DID, err)
		}
		var departments []string
		if e.Dept != "" {
			departments = []string{e.Dept}
		}
		doctors = append(doctors, Doctor{
			Key:         "mmh:" + e.DID,
			Name:        name,
			City:        mmhCity,
			Hospital:    mmhHospital,
			Departments: departments,
			Education:   education,
			URL:         u,
		})
	}
	return doctors, nil
}
