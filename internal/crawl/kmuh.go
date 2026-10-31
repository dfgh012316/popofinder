package crawl

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var (
	kmuhGroupRe     = regexp.MustCompile(`id="groupDiv_([A-Za-z0-9]+)"`)
	kmuhDeptRe      = regexp.MustCompile(`(?s)location\.href='/Web/WebRegistration/Ajax/VirtualDept\?virtualCode=([A-Za-z0-9]+)&amp;JsonLoc=([^&']+)&amp;lang=tw'[^>]*>\s*<span[^>]*>(.*?)</span>`)
	kmuhDoctorRe    = regexp.MustCompile(`DocDetail\?lang=tw&amp;doctorID=([A-Za-z0-9]+)&amp;virtualCode=([A-Za-z0-9]+)&amp;dCode=([A-Za-z0-9]+)"`)
	kmuhNameRe      = regexp.MustCompile(`(?s)<span id="dCode" class="[^"]*docDetailDocN[^"]*"[^>]*>(.*?)</span>`)
	kmuhEducationRe = regexp.MustCompile(`(?s)docDetailTitle"[^>]*>\s*學歷\s*</span>\s*</div>\s*<div class="row docContent">\s*<span[^>]*>(.*?)</span>`)
)

type kmuhHospital struct {
	base     string // scheme://host
	hospital string
	city     string
}

// Kmuh crawls the 高醫 hospitals that publish doctors in its registration
// system, each on its own host: 醫師介紹 groups → departments → each
// department's doctor list → profiles. The site's own pages load a group's
// departments with a POST; the same URL answers a GET. 小港 runs the same system
// with every department list empty, so it has its own Source (Kmsh).
type Kmuh struct {
	hospitals []kmuhHospital
	fetcher   *Fetcher
}

// NewKmuh takes the bases of 中和 (the main hospital) and 岡山, crawled in that
// order.
func NewKmuh(main, gangshan string, f *Fetcher) *Kmuh {
	return &Kmuh{fetcher: f, hospitals: []kmuhHospital{
		{main, "高雄醫學大學附設中和紀念醫院", "高雄"},
		{gangshan, "高醫岡山醫院", "高雄"},
	}}
}

func (s *Kmuh) Name() string { return "kmuh" }

type kmuhDept struct {
	VirtualCode, JsonLoc, Name string
}

// parseKmuhGroups returns the 醫師介紹 group codes in page order.
func parseKmuhGroups(page string) []string {
	var groups []string
	for _, m := range kmuhGroupRe.FindAllStringSubmatch(page, -1) {
		groups = append(groups, m[1])
	}
	return groups
}

// parseKmuhDepts returns a group's departments in page order. JsonLoc stays
// URL-escaped as the page writes it.
func parseKmuhDepts(page string) []kmuhDept {
	var depts []kmuhDept
	for _, m := range kmuhDeptRe.FindAllStringSubmatch(page, -1) {
		if name := htmlText(m[3]); name != "" {
			depts = append(depts, kmuhDept{VirtualCode: m[1], JsonLoc: m[2], Name: name})
		}
	}
	return depts
}

// parseKmuhDoctorList returns the doctor IDs on a department's doctor list in
// page order, deduped, with the profile query ("virtualCode=…&dCode=…") of
// each one's first link.
func parseKmuhDoctorList(page string) (ids []string, query map[string]string) {
	query = map[string]string{}
	for _, m := range kmuhDoctorRe.FindAllStringSubmatch(page, -1) {
		if _, ok := query[m[1]]; !ok {
			query[m[1]] = "virtualCode=" + m[2] + "&dCode=" + m[3]
			ids = append(ids, m[1])
		}
	}
	return ids, query
}

// parseKmuhProfile reads the name ("<姓名> 醫師") and the lines of the 學歷 field;
// a profile without a 學歷 field has nil education.
func parseKmuhProfile(page string) (name string, education []string, err error) {
	if m := kmuhNameRe.FindStringSubmatch(page); m != nil {
		name = strings.TrimSuffix(strings.Join(strings.Fields(htmlText(m[1])), ""), "醫師")
	}
	if name == "" {
		return "", nil, fmt.Errorf("crawl: kmuh profile without name")
	}
	if m := kmuhEducationRe.FindStringSubmatch(page); m != nil {
		education = htmlLines(m[1])
	}
	return name, education, nil
}

// Crawl crawls the hospitals in order. A doctor ID listed at more than one of
// them is kept once, under the first. Any failed page fails the whole crawl,
// except unavailable profiles (some redirect to the home page; see profileSkips).
func (s *Kmuh) Crawl(ctx context.Context) ([]Doctor, error) {
	taken := map[string]bool{}
	skips := &profileSkips{}
	var doctors []Doctor
	for _, h := range s.hospitals {
		ds, err := s.crawlHospital(ctx, h, taken, skips)
		if err != nil {
			return nil, err
		}
		doctors = append(doctors, ds...)
	}
	skips.log(s.Name())
	return doctors, nil
}

// crawlHospital returns the hospital's doctors not in taken, with departments
// from this hospital only, and adds every profile it requests to taken. A
// hospital listing no doctor at all is an error even if all of them are taken
// (see Cgmh.crawlCampus).
func (s *Kmuh) crawlHospital(ctx context.Context, h kmuhHospital, taken map[string]bool, skips *profileSkips) ([]Doctor, error) {
	intro, err := s.fetcher.Get(ctx, h.base+"/Web/WebRegistration/DocIntro/Intro?lang=tw")
	if err != nil {
		return nil, fmt.Errorf("crawl: kmuh %s: %w", h.base, err)
	}
	groups := parseKmuhGroups(intro)
	if len(groups) == 0 {
		return nil, fmt.Errorf("crawl: kmuh %s has no groups", h.base)
	}

	x := newDoctorIndex()
	seenDept := map[string]bool{}
	found := 0
	for _, g := range groups {
		page, err := s.fetcher.Get(ctx, h.base+"/Web/WebRegistration/Ajax/VirtualDept?lang=tw&groupCode="+g+"&Controller=DocIntro&Action=ShowDocList")
		if err != nil {
			return nil, fmt.Errorf("crawl: kmuh %s group %s: %w", h.base, g, err)
		}
		for _, d := range parseKmuhDepts(page) {
			if seenDept[d.VirtualCode] {
				continue
			}
			seenDept[d.VirtualCode] = true
			list, err := s.fetcher.Get(ctx, h.base+"/Web/WebRegistration/DocIntro/ShowDocList?virtualCode="+d.VirtualCode+"&JsonLoc="+d.JsonLoc+"&lang=tw")
			if err != nil {
				return nil, fmt.Errorf("crawl: kmuh %s department %s: %w", h.base, d.VirtualCode, err)
			}
			ids, query := parseKmuhDoctorList(list)
			for _, id := range ids {
				found++
				if !taken[id] {
					x.add(id, query[id], d.Name)
				}
			}
		}
	}
	if found == 0 {
		return nil, fmt.Errorf("crawl: kmuh %s found no doctors", h.base)
	}

	doctors := make([]Doctor, 0, len(x.order))
	for _, id := range x.order {
		taken[id] = true
		u := h.base + "/Web/WebRegistration/DocIntro/DocDetail?lang=tw&doctorID=" + id + "&" + x.first[id]
		page, err := s.fetcher.Get(ctx, u)
		skipped := profileUnavailable(err)
		if err != nil && !skipped {
			return nil, fmt.Errorf("crawl: kmuh doctor %s: %w", id, err)
		}
		if err := skips.add(skipped); err != nil {
			return nil, fmt.Errorf("crawl: kmuh: %w", err)
		}
		if skipped {
			continue
		}
		name, education, err := parseKmuhProfile(page)
		if err != nil {
			return nil, fmt.Errorf("crawl: kmuh doctor %s: %w", id, err)
		}
		doctors = append(doctors, Doctor{
			Key:         "kmuh:" + id,
			Name:        name,
			City:        h.city,
			Hospital:    h.hospital,
			Departments: x.depts[id],
			Education:   education,
			URL:         u,
		})
	}
	return doctors, nil
}
