package crawl

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var (
	// Departments are <h4> links; a group's sub-departments are <h5> links under a plain <h4> heading.
	cgmhDeptLinkRe  = regexp.MustCompile(`(?s)<a href="/tw/Services/DeptInfo/([^"]+)"[^>]*>\s*<h[45]>(.*?)</h[45]>\s*</a>`)
	cgmhDoctorIDRe  = regexp.MustCompile(`href="/tw/Services/DoctorInfo/([A-Za-z0-9]+)"`)
	cgmhNameRe      = regexp.MustCompile(`(?s)<h3 class="fz-32">(.*?)</h3>`)
	cgmhEducationRe = regexp.MustCompile(`(?s)<div class="fz-24 fwb">\s*學歷\s*</div>\s*<ul[^>]*>(.*?)</ul>`)
)

type cgmhCampus struct {
	code     string // DeptList/<code>
	hospital string
	city     string
}

// Cgmh crawls 長庚 (www.cgmh.org.tw, every campus on one site). A doctor's
// DoctorInfo id is the same at every campus, so a doctor listed at several
// campuses is kept once, under the first campus in this order.
type Cgmh struct {
	base     string
	campuses []cgmhCampus
	fetcher  *Fetcher
}

func NewCgmh(base string, f *Fetcher) *Cgmh {
	return &Cgmh{base: base, fetcher: f, campuses: []cgmhCampus{
		{"3", "林口長庚紀念醫院", "桃園"},
		{"1", "台北長庚紀念醫院", "台北"},
		{"2", "基隆長庚紀念醫院", "基隆"},
		{"5", "桃園長庚紀念醫院", "桃園"},
		{"6", "嘉義長庚紀念醫院", "嘉義"},
		{"M", "雲林長庚紀念醫院", "雲林"},
		{"8", "高雄長庚紀念醫院", "高雄"},
		{"V", "新北市立土城醫院", "新北"},
		{"T", "高雄市立鳳山醫院", "高雄"},
		{"W", "高雄市立大同醫院", "高雄"},
	}}
}

func (s *Cgmh) Name() string { return "cgmh" }

func (s *Cgmh) profileURL(id string) string { return s.base + "/tw/Services/DoctorInfo/" + id }

type cgmhDept struct {
	Path, Name string // Path is relative to /tw/Services/DeptInfo/
}

// parseCgmhDeptList returns the department pages linked from a campus's DeptList
// in page order, deduped by path, without links that have no name.
func parseCgmhDeptList(page string) []cgmhDept {
	var depts []cgmhDept
	seen := map[string]bool{}
	for _, m := range cgmhDeptLinkRe.FindAllStringSubmatch(page, -1) {
		path, name := m[1], htmlText(m[2])
		if name == "" || seen[path] {
			continue
		}
		seen[path] = true
		depts = append(depts, cgmhDept{Path: path, Name: name})
	}
	return depts
}

// parseCgmhDeptPage returns the DoctorInfo ids on a department page in page order.
func parseCgmhDeptPage(page string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, m := range cgmhDoctorIDRe.FindAllStringSubmatch(page, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	return ids
}

// parseCgmhProfile reads the name from the profile heading ("<姓名>醫師") and the
// lines of the 學歷 list; a profile without a 學歷 list has nil education.
func parseCgmhProfile(page string) (name string, education []string, err error) {
	if m := cgmhNameRe.FindStringSubmatch(page); m != nil {
		name = strings.TrimSuffix(strings.Join(strings.Fields(htmlText(m[1])), ""), "醫師")
	}
	if name == "" {
		return "", nil, fmt.Errorf("crawl: cgmh profile without name")
	}
	if m := cgmhEducationRe.FindStringSubmatch(page); m != nil {
		for _, li := range listItemRe.FindAllStringSubmatch(m[1], -1) {
			if s := htmlText(li[1]); s != "" {
				education = append(education, s)
			}
		}
	}
	return name, education, nil
}

type cgmhProgress struct {
	taken map[string]bool // profiles already requested under an earlier campus
	skips profileSkips
}

// Crawl crawls the campuses in order. Any failed page fails the whole crawl,
// except unavailable profiles (mostly 500s, mostly doctors listed as 職位「醫師」;
// see profileSkips).
func (s *Cgmh) Crawl(ctx context.Context) ([]Doctor, error) {
	p := &cgmhProgress{taken: map[string]bool{}}
	var doctors []Doctor
	for _, c := range s.campuses {
		ds, err := s.crawlCampus(ctx, c, p)
		if err != nil {
			return nil, err
		}
		doctors = append(doctors, ds...)
	}
	p.skips.log(s.Name())
	return doctors, nil
}

// crawlCampus returns the campus's doctors whose profiles were not requested under
// an earlier campus, with departments from this campus only.
// A campus listing no doctor at all is an error even if all of them are taken:
// the delete guard counts the whole source, so a small campus emptied by a site
// change would otherwise lose its doctors.
func (s *Cgmh) crawlCampus(ctx context.Context, c cgmhCampus, p *cgmhProgress) ([]Doctor, error) {
	list, err := s.fetcher.Get(ctx, s.base+"/tw/Services/DeptList/"+c.code)
	if err != nil {
		return nil, fmt.Errorf("crawl: cgmh campus %s: %w", c.code, err)
	}
	depts := parseCgmhDeptList(list)
	if len(depts) == 0 {
		return nil, fmt.Errorf("crawl: cgmh campus %s has no departments", c.code)
	}

	x := newDoctorIndex()
	found := 0
	for _, d := range depts {
		page, err := s.fetcher.Get(ctx, s.base+"/tw/Services/DeptInfo/"+d.Path)
		if err != nil {
			return nil, fmt.Errorf("crawl: cgmh department %s: %w", d.Path, err)
		}
		for _, id := range parseCgmhDeptPage(page) {
			found++
			if !p.taken[id] {
				x.add(id, d.Path, d.Name)
			}
		}
	}
	if found == 0 {
		return nil, fmt.Errorf("crawl: cgmh campus %s found no doctors", c.code)
	}

	doctors := make([]Doctor, 0, len(x.order))
	for _, id := range x.order {
		p.taken[id] = true
		u := s.profileURL(id)
		page, err := s.fetcher.Get(ctx, u)
		skipped := profileUnavailable(err)
		if err != nil && !skipped {
			return nil, fmt.Errorf("crawl: cgmh doctor %s: %w", id, err)
		}
		if err := p.skips.add(skipped); err != nil {
			return nil, fmt.Errorf("crawl: cgmh: %w", err)
		}
		if skipped {
			continue
		}
		name, education, err := parseCgmhProfile(page)
		if err != nil {
			return nil, fmt.Errorf("crawl: cgmh doctor %s: %w", id, err)
		}
		doctors = append(doctors, Doctor{
			Key:         "cgmh:" + id,
			Name:        name,
			City:        c.city,
			Hospital:    c.hospital,
			Departments: x.depts[id],
			Education:   education,
			URL:         u,
		})
	}
	return doctors, nil
}
