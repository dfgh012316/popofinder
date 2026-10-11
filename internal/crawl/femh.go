package crawl

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"
)

const (
	femhHospital = "亞東紀念醫院"
	femhCity     = "新北"
)

var (
	// Doctors in these units are also listed on their own department pages, so the units are skipped.
	femhExcludedDept = regexp.MustCompile("加護病房|呼吸照護|健康管理")

	femhIndexLinkRe = regexp.MustCompile(`(?s)<a[^>]*href=["'][^"']*sectionDetail\.aspx\?CID=(\w+)["'][^>]*>(.*?)</a>`)
	femhDoctorIDRe  = regexp.MustCompile(`sectionDetail2\.aspx\?CID=\w+(?:&amp;|&)+DoctorID=(\w+)`)
	femhTitleRe     = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
	femhEducationRe = regexp.MustCompile(`(?s)class=["']h5 color-g name["']>\s*學歷\s*</div>\s*<ul>(.*?)</ul>`)
	femhItemRe      = regexp.MustCompile(`(?s)<li>(.*?)</li>`)
	femhTagRe       = regexp.MustCompile(`<[^>]*>`)
)

// Femh crawls 亞東紀念醫院 (www.femh.org.tw): index → department pages → profile pages.
type Femh struct {
	baseURL string
	fetcher *Fetcher
}

func NewFemh(baseURL string, f *Fetcher) *Femh {
	return &Femh{baseURL: baseURL, fetcher: f}
}

func (s *Femh) Name() string { return "femh" }

type femhDept struct {
	CID, Name string
}

func (s *Femh) indexURL() string { return s.baseURL + "/section/section1.aspx" }

func (s *Femh) deptURL(cid string) string {
	return s.baseURL + "/section/sectionDetail.aspx?CID=" + cid
}

func (s *Femh) profileURL(cid, doctorID string) string {
	return s.baseURL + "/section/sectionDetail2.aspx?CID=" + cid + "&&DoctorID=" + doctorID
}

func femhText(raw string) string {
	return strings.TrimSpace(html.UnescapeString(femhTagRe.ReplaceAllString(raw, "")))
}

// parseFemhIndex returns the departments linked from the index in page order,
// deduped by CID and without the excluded units.
func parseFemhIndex(page string) []femhDept {
	var depts []femhDept
	seen := map[string]bool{}
	for _, m := range femhIndexLinkRe.FindAllStringSubmatch(page, -1) {
		cid, name := m[1], femhText(m[2])
		if seen[cid] {
			continue
		}
		seen[cid] = true
		if femhExcludedDept.MatchString(name) {
			continue
		}
		depts = append(depts, femhDept{CID: cid, Name: name})
	}
	return depts
}

// parseFemhDept returns the DoctorIDs on a department page in page order.
func parseFemhDept(page string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, m := range femhDoctorIDRe.FindAllStringSubmatch(page, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	return ids
}

// parseFemhProfile reads the name from <title> ("姓名醫師｜科別｜亞東紀念醫院") and
// the lines of the 學歷 block; a profile without a 學歷 block has nil education.
func parseFemhProfile(page string) (name string, education []string, err error) {
	if m := femhTitleRe.FindStringSubmatch(page); m != nil {
		title := html.UnescapeString(m[1])
		if before, _, ok := strings.Cut(title, "醫師｜"); ok {
			name = before
		} else {
			name, _, _ = strings.Cut(title, "｜")
		}
		name = strings.Join(strings.Fields(name), "")
	}
	if name == "" {
		return "", nil, fmt.Errorf("crawl: femh profile without name")
	}
	if m := femhEducationRe.FindStringSubmatch(page); m != nil {
		for _, li := range femhItemRe.FindAllStringSubmatch(m[1], -1) {
			if s := femhText(li[1]); s != "" {
				education = append(education, s)
			}
		}
	}
	return name, education, nil
}

// Crawl returns every doctor once, with departments in index order. Any failed
// page fails the whole crawl.
func (s *Femh) Crawl(ctx context.Context) ([]Doctor, error) {
	index, err := s.fetcher.Get(ctx, s.indexURL())
	if err != nil {
		return nil, fmt.Errorf("crawl: femh index: %w", err)
	}
	depts := parseFemhIndex(index)
	if len(depts) == 0 {
		return nil, fmt.Errorf("crawl: femh index has no departments")
	}

	type doctorDepts struct {
		firstCID string
		names    []string
	}
	var order []string
	byID := map[string]*doctorDepts{}
	for _, d := range depts {
		page, err := s.fetcher.Get(ctx, s.deptURL(d.CID))
		if err != nil {
			return nil, fmt.Errorf("crawl: femh department %s: %w", d.CID, err)
		}
		for _, id := range parseFemhDept(page) {
			dd, ok := byID[id]
			if !ok {
				dd = &doctorDepts{firstCID: d.CID}
				byID[id] = dd
				order = append(order, id)
			}
			dd.names = append(dd.names, d.Name)
		}
	}
	if len(order) == 0 {
		return nil, fmt.Errorf("crawl: femh found no doctors")
	}

	doctors := make([]Doctor, 0, len(order))
	for _, id := range order {
		dd := byID[id]
		u := s.profileURL(dd.firstCID, id)
		page, err := s.fetcher.Get(ctx, u)
		if err != nil {
			return nil, fmt.Errorf("crawl: femh doctor %s: %w", id, err)
		}
		name, education, err := parseFemhProfile(page)
		if err != nil {
			return nil, fmt.Errorf("crawl: femh doctor %s: %w", id, err)
		}
		doctors = append(doctors, Doctor{
			Key:         "femh:" + id,
			Name:        name,
			City:        femhCity,
			Hospital:    femhHospital,
			Departments: dd.names,
			Education:   education,
			URL:         u,
		})
	}
	return doctors, nil
}
