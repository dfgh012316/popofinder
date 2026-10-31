package crawl

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var (
	kmshDeptRe   = regexp.MustCompile(`(?s)href="doc_index\.asp\?dept_code=(\d+)\s*"[^>]*>(.*?)</a>`)
	kmshDoctorRe = regexp.MustCompile(`href="(search_data\d*\.asp\?pno=(\d+)&dept_code=\d+)"`)
	kmshNameRe   = regexp.MustCompile(`<li><font size="3">姓名：([^<]*)</font>`)
	// The 學　歷 header row (with two empty cells) is followed by one full-width row.
	kmshEducationRe = regexp.MustCompile(`(?s)學[　\s]*歷</font></b></td>(?:\s*<td[^>]*>\s*</td>)*\s*</tr>\s*<tr>\s*<td[^>]*colspan="3"[^>]*>\s*<font[^>]*>(.*?)</font>`)
)

// Kmsh crawls 高雄市立小港醫院 (operated by 高醫). Its registration system has the
// same 醫師介紹 as Kmuh with every department list empty; the doctors are on the
// old site instead: department index → each department's doctors → profiles.
// pno is that site's own doctor number, unrelated to Kmuh's doctor IDs, so a
// doctor listed at both hospitals has one row for each.
type Kmsh struct {
	base    string // the old site's doctor pages, e.g. …/2024KMSH/pro
	fetcher *Fetcher
}

// NewKmsh takes the directory of the old site's doctor pages: …/wwwKMHK/pro/doc.asp
// redirects to …/dept/8300/2024KMSH/pro/doc.asp, and the Fetcher does not follow
// redirects.
func NewKmsh(base string, f *Fetcher) *Kmsh { return &Kmsh{base: base, fetcher: f} }

func (s *Kmsh) Name() string { return "kmsh" }

type kmshDept struct{ Code, Name string }

// parseKmshDepts returns the departments on the index in page order, deduped.
func parseKmshDepts(page string) []kmshDept {
	var depts []kmshDept
	seen := map[string]bool{}
	for _, m := range kmshDeptRe.FindAllStringSubmatch(page, -1) {
		name := htmlText(m[2])
		if name == "" || seen[m[1]] {
			continue
		}
		seen[m[1]] = true
		depts = append(depts, kmshDept{Code: m[1], Name: name})
	}
	return depts
}

// parseKmshDoctors returns the pno of each doctor on a department page in page
// order, deduped, with the profile path of each one's first link.
func parseKmshDoctors(page string) (pnos []string, path map[string]string) {
	path = map[string]string{}
	for _, m := range kmshDoctorRe.FindAllStringSubmatch(page, -1) {
		if _, ok := path[m[2]]; !ok {
			path[m[2]] = m[1]
			pnos = append(pnos, m[2])
		}
	}
	return pnos, path
}

// parseKmshProfile reads the name from 「姓名：」 and the lines under 學　歷; a
// profile without that field has nil education.
func parseKmshProfile(page string) (name string, education []string, err error) {
	if m := kmshNameRe.FindStringSubmatch(page); m != nil {
		name = strings.Join(strings.Fields(htmlText(m[1])), "")
	}
	if name == "" {
		return "", nil, fmt.Errorf("crawl: kmsh profile without name")
	}
	if m := kmshEducationRe.FindStringSubmatch(page); m != nil {
		education = htmlLines(m[1])
	}
	return name, education, nil
}

// Crawl returns every doctor listed under a department. Any failed page fails
// the whole crawl, and so does finding no doctor at all.
func (s *Kmsh) Crawl(ctx context.Context) ([]Doctor, error) {
	index, err := s.fetcher.Get(ctx, s.base+"/doc.asp")
	if err != nil {
		return nil, fmt.Errorf("crawl: kmsh index: %w", err)
	}
	depts := parseKmshDepts(index)
	if len(depts) == 0 {
		return nil, fmt.Errorf("crawl: kmsh index has no departments")
	}

	x := newDoctorIndex()
	for _, d := range depts {
		page, err := s.fetcher.Get(ctx, s.base+"/doc_index.asp?dept_code="+d.Code)
		if err != nil {
			return nil, fmt.Errorf("crawl: kmsh department %s: %w", d.Code, err)
		}
		pnos, path := parseKmshDoctors(page)
		for _, pno := range pnos {
			x.add(pno, path[pno], d.Name)
		}
	}
	if len(x.order) == 0 {
		return nil, fmt.Errorf("crawl: kmsh found no doctors")
	}

	doctors := make([]Doctor, 0, len(x.order))
	for _, pno := range x.order {
		u := s.base + "/" + x.first[pno]
		page, err := s.fetcher.Get(ctx, u)
		if err != nil {
			return nil, fmt.Errorf("crawl: kmsh doctor %s: %w", pno, err)
		}
		name, education, err := parseKmshProfile(page)
		if err != nil {
			return nil, fmt.Errorf("crawl: kmsh doctor %s: %w", pno, err)
		}
		doctors = append(doctors, Doctor{
			Key:         "kmsh:" + pno,
			Name:        name,
			City:        "高雄",
			Hospital:    "高雄市立小港醫院",
			Departments: x.depts[pno],
			Education:   education,
			URL:         u,
		})
	}
	return doctors, nil
}
