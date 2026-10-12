package crawl

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Non-doctor units on the branch indexes.
	cghExcludedDept = regexp.MustCompile("護理|藥劑|檢驗|營養|人事|公關")

	cghCommentRe    = regexp.MustCompile(`(?s)<!--.*?-->`)
	cghListLinkRe   = regexp.MustCompile(`(?s)<a[^>]*href="category_doct\.asp\?mode=listView&(?:amp;)?category_id=(\d+)"[^>]*>(.*?)</a>`)
	cghBranchDeptRe = regexp.MustCompile(`(?s)(?:<a class="[^"]*btn_division[^"]*"|<div class="btn_division">\s*<a) href="([^"]*?category\.asp\?category_id=(\d+))"[^>]*>(.*?)</a>`)
	cghDoctorIDRe   = regexp.MustCompile(`product_doct\.asp\?prodid=(\d+)&(?:amp;)?category_id=\d+`)
	cghLastPageRe   = regexp.MustCompile(`<a href='[^']*[?&]pageno=(\d+)[^']*'>&ge;</a>`)
	cghNameRe       = regexp.MustCompile(`(?s)<span id="pd_name">(.*?)</span>`)
	cghEducationRe  = regexp.MustCompile(`(?s)<h2 class="cgh2_title">\s*學歷\s*</h2>\s*<ul>(.*?)</ul>`)
)

type cghCampus struct {
	host     string // scheme://host
	base     string // path prefix, e.g. "/ec99/rwd1320/"
	index    string // index page, relative to base
	branch   bool   // the index links to department pages, which link to the doctor lists
	hospital string
	city     string
}

func (c cghCampus) url(rel string) string { return c.host + c.base + rel }

func (c cghCampus) listURL(listID string, page int) string {
	return c.url(fmt.Sprintf("category_doct.asp?mode=listView&category_id=%s&pageno=%d", listID, page))
}

func (c cghCampus) profileURL(prodID, listID string) string {
	return c.url("product_doct.asp?prodid=" + prodID + "&category_id=" + listID)
}

// Cgh crawls 國泰 (www/sijhih/hsinchu/neihu.cgh.org.tw, one CMS). A doctor listed
// at several campuses shares one prodid and is kept once, under the first campus.
type Cgh struct {
	campuses []cghCampus
	fetcher  *Fetcher
}

func NewCgh(mainHost, sijhihHost, hsinchuHost, neihuHost string, f *Fetcher) *Cgh {
	return &Cgh{fetcher: f, campuses: []cghCampus{
		{mainHost, "/ec99/rwd1320/", "category.asp?category_id=1553", false, "國泰綜合醫院", "台北"},
		{sijhihHost, "/ec99/rwd102/", "category.asp?category_id=2", true, "汐止國泰綜合醫院", "新北"},
		{hsinchuHost, "/ec99/rwd103/", "category.asp?category_id=2", true, "新竹國泰綜合醫院", "新竹"},
		{neihuHost, "/ec99/rwd1309A/", "category.asp?category_id=45", true, "內湖國泰診所", "台北"},
	}}
}

func (s *Cgh) Name() string { return "cgh" }

type cghDept struct {
	ID, Name string
}

// parseCghIndex returns the doctor lists linked from the main campus index in
// page order, deduped by list id (keeping the first name). Commented-out links
// are dropped first.
func parseCghIndex(page string) []cghDept {
	var depts []cghDept
	seen := map[string]bool{}
	for _, m := range cghListLinkRe.FindAllStringSubmatch(cghCommentRe.ReplaceAllString(page, ""), -1) {
		if seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		depts = append(depts, cghDept{ID: m[1], Name: htmlText(m[2])})
	}
	return depts
}

// parseCghBranchIndex returns the department pages linked from a branch index in
// page order, deduped by category id, without off-site links or the excluded units.
func parseCghBranchIndex(page string) []cghDept {
	var depts []cghDept
	seen := map[string]bool{}
	for _, m := range cghBranchDeptRe.FindAllStringSubmatch(cghCommentRe.ReplaceAllString(page, ""), -1) {
		href, id, name := m[1], m[2], htmlText(m[3])
		if strings.HasPrefix(href, "http") || cghExcludedDept.MatchString(name) || seen[id] {
			continue
		}
		seen[id] = true
		depts = append(depts, cghDept{ID: id, Name: name})
	}
	return depts
}

// parseCghListIDs returns the doctor list ids linked from a branch department page.
func parseCghListIDs(page string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, m := range cghListLinkRe.FindAllStringSubmatch(cghCommentRe.ReplaceAllString(page, ""), -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	return ids
}

// parseCghListPage returns the prodids on a list page in page order and the last
// page number from the &ge; link (1 when there is none). Past the last page the
// site serves the last page again, so the page count must come from that link.
func parseCghListPage(page string) (ids []string, lastPage int) {
	seen := map[string]bool{}
	for _, m := range cghDoctorIDRe.FindAllStringSubmatch(page, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	lastPage = 1
	if m := cghLastPageRe.FindStringSubmatch(page); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			lastPage = n
		}
	}
	return ids, lastPage
}

// parseCghProfile reads the name from pd_name ("職稱：姓名(附註)"; <title> is
// always 醫師介紹) and the lines of the 學歷 list; a profile without a 學歷 list
// has nil education.
func parseCghProfile(page string) (name string, education []string, err error) {
	if m := cghNameRe.FindStringSubmatch(page); m != nil {
		n := htmlText(m[1])
		if _, after, ok := strings.Cut(n, "："); ok {
			n = after
		}
		n, _, _ = strings.Cut(n, "(")
		n, _, _ = strings.Cut(n, "（")
		name = strings.Join(strings.Fields(n), "")
	}
	if name == "" {
		return "", nil, fmt.Errorf("crawl: cgh profile without name")
	}
	if m := cghEducationRe.FindStringSubmatch(page); m != nil {
		for _, li := range listItemRe.FindAllStringSubmatch(m[1], -1) {
			if s := htmlText(li[1]); s != "" {
				education = append(education, s)
			}
		}
	}
	return name, education, nil
}

// Crawl crawls the campuses in order, one at a time, so consecutive requests go to
// the same host. Any failed page fails the whole crawl.
func (s *Cgh) Crawl(ctx context.Context) ([]Doctor, error) {
	taken := map[string]bool{}
	var doctors []Doctor
	for _, c := range s.campuses {
		ds, err := s.crawlCampus(ctx, c, taken)
		if err != nil {
			return nil, err
		}
		doctors = append(doctors, ds...)
	}
	return doctors, nil
}

// crawlCampus returns the campus's doctors not already in taken (kept under an
// earlier campus), with departments from this campus only, and adds them to taken.
// A campus listing no doctor at all is an error even if all of them are taken:
// the delete guard counts the whole source, so a small campus emptied by a site
// change would otherwise lose its doctors.
func (s *Cgh) crawlCampus(ctx context.Context, c cghCampus, taken map[string]bool) ([]Doctor, error) {
	index, err := s.fetcher.Get(ctx, c.url(c.index))
	if err != nil {
		return nil, fmt.Errorf("crawl: cgh %s index: %w", c.host, err)
	}

	var lists []cghDept
	if !c.branch {
		lists = parseCghIndex(index)
		if len(lists) == 0 {
			return nil, fmt.Errorf("crawl: cgh %s index has no departments", c.host)
		}
	} else {
		depts := parseCghBranchIndex(index)
		if len(depts) == 0 {
			return nil, fmt.Errorf("crawl: cgh %s index has no departments", c.host)
		}
		seen := map[string]bool{}
		for _, d := range depts {
			page, err := s.fetcher.Get(ctx, c.url("category.asp?category_id="+d.ID))
			if err != nil {
				return nil, fmt.Errorf("crawl: cgh %s department %s: %w", c.host, d.ID, err)
			}
			for _, id := range parseCghListIDs(page) {
				if !seen[id] {
					seen[id] = true
					lists = append(lists, cghDept{ID: id, Name: d.Name})
				}
			}
		}
	}

	x := newDoctorIndex()
	found := 0
	for _, l := range lists {
		for p, last := 1, 1; p <= last; p++ {
			page, err := s.fetcher.Get(ctx, c.listURL(l.ID, p))
			if err != nil {
				return nil, fmt.Errorf("crawl: cgh %s list %s page %d: %w", c.host, l.ID, p, err)
			}
			ids, lastPage := parseCghListPage(page)
			if p == 1 {
				last = lastPage
			}
			for _, id := range ids {
				found++
				if !taken[id] {
					x.add(id, l.ID, l.Name)
				}
			}
		}
	}
	if found == 0 {
		return nil, fmt.Errorf("crawl: cgh %s found no doctors", c.host)
	}

	doctors := make([]Doctor, 0, len(x.order))
	for _, id := range x.order {
		u := c.profileURL(id, x.first[id])
		page, err := s.fetcher.Get(ctx, u)
		if err != nil {
			return nil, fmt.Errorf("crawl: cgh doctor %s: %w", id, err)
		}
		name, education, err := parseCghProfile(page)
		if err != nil {
			return nil, fmt.Errorf("crawl: cgh doctor %s: %w", id, err)
		}
		doctors = append(doctors, Doctor{
			Key:         "cgh:" + id,
			Name:        name,
			City:        c.city,
			Hospital:    c.hospital,
			Departments: x.depts[id],
			Education:   education,
			URL:         u,
		})
		taken[id] = true
	}
	return doctors, nil
}
