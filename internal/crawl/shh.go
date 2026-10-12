package crawl

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

const (
	shhHospital = "雙和醫院"
	shhCity     = "新北"
)

var (
	// 營養、藥劑、心理諮商 are not doctors; the 健檢 and 代謝及體重 centers and the
	// 疼痛、美容醫學、睡眠障礙 clinics list doctors again under a second docCode.
	shhExcludedDept = regexp.MustCompile("營養|藥劑|心理諮商|健檢|健康檢查|代謝及體重|疼痛特別門診|美容醫學|睡眠障礙門診")

	shhIndexLinkRe = regexp.MustCompile(`(?s)<a[^>]*href=["'](?:\.\./page/|/page/)?ShhregTable\.aspx\?deptCode=(\w+)(?:&(?:amp;)?type=\d+)?["'][^>]*>(.*?)</a>`)
	shhDoctorIDRe  = regexp.MustCompile(`TeamDocDetail\.aspx\?deptCode=\w+(?:&amp;|&)docCode=(\w+)`)
	shhNameRe      = regexp.MustCompile(`(?s)<div class="text-center text-primary">\s*<h2[^>]*>(.*?)</h2>`)
	shhEducationRe = regexp.MustCompile(`(?s)<p class="title">\s*學歷\s*[：:]\s*</p>(.*?)(?:<p class="title">|</div>)`)
	shhBulletRe    = regexp.MustCompile(`(?s)<span class="point">.*?</span>`)
	shhParagraphRe = regexp.MustCompile(`(?s)<p>(.*?)</p>`)
)

// Shh crawls 雙和醫院 (shh.tmu.edu.tw): index → department pages → profile pages.
type Shh struct {
	baseURL string
	fetcher *Fetcher
}

func NewShh(baseURL string, f *Fetcher) *Shh {
	return &Shh{baseURL: baseURL, fetcher: f}
}

func (s *Shh) Name() string { return "shh" }

type shhDept struct {
	Code, Name string
}

func (s *Shh) indexURL() string { return s.baseURL + "/page/team.aspx" }

func (s *Shh) deptURL(code string) string {
	return s.baseURL + "/page/TeamDoc.aspx?deptCode=" + code
}

func (s *Shh) profileURL(code, docCode string) string {
	return s.baseURL + "/page/TeamDocDetail.aspx?deptCode=" + code + "&docCode=" + docCode
}

// parseShhIndex returns the departments linked from the index in page order,
// deduped by deptCode (keeping the first name) and without the excluded units.
func parseShhIndex(page string) []shhDept {
	var depts []shhDept
	seen := map[string]bool{}
	for _, m := range shhIndexLinkRe.FindAllStringSubmatch(page, -1) {
		code, name := m[1], htmlText(m[2])
		if seen[code] {
			continue
		}
		seen[code] = true
		if shhExcludedDept.MatchString(name) {
			continue
		}
		depts = append(depts, shhDept{Code: code, Name: name})
	}
	return depts
}

// parseShhDept returns the docCodes on a department page in page order.
func parseShhDept(page string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, m := range shhDoctorIDRe.FindAllStringSubmatch(page, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	return ids
}

// parseShhProfile reads the name from the <h2> ("姓名 職稱"; <title> is the
// hospital name) and the paragraphs of the 學歷 block; a profile without a 學歷
// block has nil education.
func parseShhProfile(page string) (name string, education []string, err error) {
	if m := shhNameRe.FindStringSubmatch(page); m != nil {
		if f := strings.Fields(htmlText(m[1])); len(f) > 0 {
			name = f[0]
		}
	}
	if name == "" {
		return "", nil, fmt.Errorf("crawl: shh profile without name")
	}
	if m := shhEducationRe.FindStringSubmatch(page); m != nil {
		block := shhBulletRe.ReplaceAllString(m[1], "")
		for _, p := range shhParagraphRe.FindAllStringSubmatch(block, -1) {
			if s := htmlText(p[1]); s != "" {
				education = append(education, s)
			}
		}
	}
	return name, education, nil
}

// Crawl returns every doctor once, with departments in index order. Any failed
// page fails the whole crawl.
func (s *Shh) Crawl(ctx context.Context) ([]Doctor, error) {
	index, err := s.fetcher.Get(ctx, s.indexURL())
	if err != nil {
		return nil, fmt.Errorf("crawl: shh index: %w", err)
	}
	depts := parseShhIndex(index)
	if len(depts) == 0 {
		return nil, fmt.Errorf("crawl: shh index has no departments")
	}

	x := newDoctorIndex()
	for _, d := range depts {
		page, err := s.fetcher.Get(ctx, s.deptURL(d.Code))
		if err != nil {
			return nil, fmt.Errorf("crawl: shh department %s: %w", d.Code, err)
		}
		for _, id := range parseShhDept(page) {
			x.add(id, d.Code, d.Name)
		}
	}
	if len(x.order) == 0 {
		return nil, fmt.Errorf("crawl: shh found no doctors")
	}

	doctors := make([]Doctor, 0, len(x.order))
	for _, id := range x.order {
		u := s.profileURL(x.first[id], id)
		page, err := s.fetcher.Get(ctx, u)
		if err != nil {
			return nil, fmt.Errorf("crawl: shh doctor %s: %w", id, err)
		}
		name, education, err := parseShhProfile(page)
		if err != nil {
			return nil, fmt.Errorf("crawl: shh doctor %s: %w", id, err)
		}
		doctors = append(doctors, Doctor{
			Key:         "shh:" + id,
			Name:        name,
			City:        shhCity,
			Hospital:    shhHospital,
			Departments: x.depts[id],
			Education:   education,
			URL:         u,
		})
	}
	return doctors, nil
}
