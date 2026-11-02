package crawl

import (
	"context"
	"fmt"
	"regexp"
)

var (
	// vghtcTokenRe matches, in page order, a group heading or a department link on
	// the registration system's department list.
	vghtcTokenRe     = regexp.MustCompile(`(?s)<th[^>]*>(.*?)</th>|listDoctor\.jsp\?init=sub&(?:amp;)?section=([A-Za-z0-9]+)"[^>]*>(.*?)</a>`)
	vghtcDoctorRe    = regexp.MustCompile(`showDoc\('([A-Za-z0-9]+)',`)
	vghtcNameRe      = regexp.MustCompile(`name="doctorChineseName" value="([^"]*)"`)
	vghtcEducationRe = regexp.MustCompile(`(?s)<h5[^>]*>\s*學歷\s*</h5>\s*<p[^>]*>(.*?)</p>`)
)

// vghtcDetails are the profile's 主治專長、經歷 (OD-18).
var vghtcDetails = detailFields{
	specialty:  regexp.MustCompile(`(?s)<h5[^>]*>\s*主治專長\s*</h5>\s*(?:<!--.*?-->\s*)*<p[^>]*>(.*?)</p>`),
	experience: regexp.MustCompile(`(?s)<h5[^>]*>\s*經歷\s*</h5>\s*<p[^>]*>(.*?)</p>`),
}

// vghtcSpecialGroup is the department list's group of special clinics. Its
// departments are crawled last, so a doctor's own department comes first and the
// 「同院同科醫師」 button searches with it.
const vghtcSpecialGroup = "特別門診"

// Vghtc crawls 臺中榮民總醫院 through its registration system, which needs no
// JavaScript (the main site's doctor pages do): department list → each
// department's doctors (from its clinic schedule) → profiles. Only doctors with
// clinics are listed. The branch hospitals (嘉義, 灣橋, 埔里) have their own
// systems and are not crawled.
type Vghtc struct {
	base    string // the registration system, e.g. https://register.vghtc.gov.tw/register
	fetcher *Fetcher
}

func NewVghtc(base string, f *Fetcher) *Vghtc { return &Vghtc{base: base, fetcher: f} }

func (s *Vghtc) Name() string { return "vghtc" }

type vghtcDept struct{ Code, Name string }

// parseVghtcDepts returns the departments on the list in page order, deduped,
// with the special clinics' group moved to the end.
func parseVghtcDepts(page string) []vghtcDept {
	var regular, special []vghtcDept
	seen := map[string]bool{}
	group := ""
	for _, m := range vghtcTokenRe.FindAllStringSubmatch(page, -1) {
		if m[2] == "" {
			group = htmlText(m[1])
			continue
		}
		name := htmlText(m[3])
		if name == "" || seen[m[2]] {
			continue
		}
		seen[m[2]] = true
		d := vghtcDept{Code: m[2], Name: name}
		if group == vghtcSpecialGroup {
			special = append(special, d)
		} else {
			regular = append(regular, d)
		}
	}
	return append(regular, special...)
}

// parseVghtcDoctors returns the doctor IDs on a department's schedule in page
// order, deduped (a doctor with several clinics is listed several times).
func parseVghtcDoctors(page string) []string {
	var ids []string
	seen := map[string]bool{}
	for _, m := range vghtcDoctorRe.FindAllStringSubmatch(page, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			ids = append(ids, m[1])
		}
	}
	return ids
}

// parseVghtcProfile reads the Chinese name and the lines of the 學歷 field; a
// profile without a 學歷 field has nil education.
func parseVghtcProfile(page string) (name string, education []string, err error) {
	if m := vghtcNameRe.FindStringSubmatch(page); m != nil {
		name = htmlText(m[1])
	}
	if name == "" {
		return "", nil, fmt.Errorf("crawl: vghtc profile without name")
	}
	if m := vghtcEducationRe.FindStringSubmatch(page); m != nil {
		education = htmlLines(m[1])
	}
	return name, education, nil
}

// Crawl fails on any failed page.
func (s *Vghtc) Crawl(ctx context.Context) ([]Doctor, error) {
	list, err := s.fetcher.Get(ctx, s.base+"/listSection.jsp")
	if err != nil {
		return nil, fmt.Errorf("crawl: vghtc department list: %w", err)
	}
	depts := parseVghtcDepts(list)
	if len(depts) == 0 {
		return nil, fmt.Errorf("crawl: vghtc has no departments")
	}

	x := newDoctorIndex()
	for _, d := range depts {
		page, err := s.fetcher.Get(ctx, s.base+"/regDoctor.jsp?init=sub&section="+d.Code)
		if err != nil {
			return nil, fmt.Errorf("crawl: vghtc department %s: %w", d.Code, err)
		}
		for _, id := range parseVghtcDoctors(page) {
			x.add(id, d.Code, d.Name)
		}
	}
	if len(x.order) == 0 {
		return nil, fmt.Errorf("crawl: vghtc found no doctors")
	}

	doctors := make([]Doctor, 0, len(x.order))
	for _, id := range x.order {
		u := s.base + "/regDocSchedule.jsp?init=init&section=" + x.first[id] + "&drno=" + id
		page, err := s.fetcher.Get(ctx, u)
		if err != nil {
			return nil, fmt.Errorf("crawl: vghtc doctor %s: %w", id, err)
		}
		name, education, err := parseVghtcProfile(page)
		if err != nil {
			return nil, fmt.Errorf("crawl: vghtc doctor %s: %w", id, err)
		}
		specialty, experience := vghtcDetails.read(page)
		doctors = append(doctors, Doctor{
			Key:         "vghtc:" + id,
			Name:        name,
			City:        "台中",
			Hospital:    "臺中榮民總醫院",
			Departments: x.depts[id],
			Education:   education,
			Specialty:   specialty,
			Experience:  experience,
			URL:         u,
		})
	}
	return doctors, nil
}
