package crawl

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

var (
	smileDoctorLinkRe = regexp.MustCompile(`href="[^"]*/doctors/([a-z0-9-]+)"`)
	smileShortlinkRe  = regexp.MustCompile(`rel=['"]shortlink['"] href=['"][^'"]*\?p=(\d+)['"]`)
	// The profile's first section holds the name heading and the bio, written
	// either as a <p> right after the heading or inside a rich-text <div>.
	smileBioRe = regexp.MustCompile(`(?s)class="page-wrapper">.*?<h2[^>]*>(.*?)</h2>(.*?)</section>`)
	// The bio mixes schools, posts and certifications in one list: a school line
	// names a school and no post.
	smileSchoolRe = regexp.MustCompile(`(?i)大學|學院|university|college`)
	smilePostRe   = regexp.MustCompile(`醫院|院長|主任|主治|教授|講師|醫師|研究員|住院|顧問|理事|會員|現任|隊長`)
)

type smileBranch struct {
	slug     string // all-doctors/<slug>
	hospital string
	city     string
}

// Smile crawls 蒔美牙醫 (www.smile-dental.tw, one WordPress site for every
// branch): each branch's doctor list → profiles. A doctor working at several
// branches is kept once, under the first branch in this order.
type Smile struct {
	base     string
	branches []smileBranch
	fetcher  *Fetcher
}

func NewSmile(base string, f *Fetcher) *Smile {
	return &Smile{base: base, fetcher: f, branches: []smileBranch{
		{"banqiaosmile", "板橋蒔美牙醫診所", "新北"},
		{"jinshansmile", "金山蒔美牙醫診所", "台北"},
		{"sanchongsmile", "三重蒔美牙醫診所", "新北"},
		{"xizhismile", "汐止蒔美牙醫診所", "新北"},
		{"beidasmile", "北大蒔美牙醫診所", "新北"},
	}}
}

func (s *Smile) Name() string { return "smile" }

// parseSmileBranch returns the profile slugs on a branch's doctor list in page
// order, deduped.
func parseSmileBranch(page string) []string {
	var slugs []string
	seen := map[string]bool{}
	for _, m := range smileDoctorLinkRe.FindAllStringSubmatch(page, -1) {
		if !seen[m[1]] {
			seen[m[1]] = true
			slugs = append(slugs, m[1])
		}
	}
	return slugs
}

// parseSmileProfile reads the WordPress post ID (the slug is the doctor's name,
// so it is not a stable key), the name heading ("<姓名>醫師") and the school
// lines of the bio. A bio without school lines has nil education.
func parseSmileProfile(page string) (id, name string, education []string, err error) {
	if m := smileShortlinkRe.FindStringSubmatch(page); m != nil {
		id = m[1]
	}
	m := smileBioRe.FindStringSubmatch(page)
	if m != nil {
		name = strings.TrimSuffix(strings.Join(strings.Fields(htmlText(m[1])), ""), "醫師")
	}
	if id == "" || name == "" {
		return "", "", nil, fmt.Errorf("crawl: smile profile without post ID or name")
	}
	for _, line := range htmlLines(m[2]) {
		for _, part := range strings.Split(line, "、") {
			if part = strings.TrimSpace(part); smileSchoolRe.MatchString(part) && !smilePostRe.MatchString(part) {
				education = append(education, part)
			}
		}
	}
	return id, name, education, nil
}

// smileProfileError names a failed profile by its position on the branch's
// doctor list and drops the slug, which spells the doctor's name, from err (the
// fetcher's errors carry the URL): crawl logs carry site IDs only (OD-10).
func smileProfileError(branch string, i int, slug string, err error) error {
	return fmt.Errorf("crawl: smile branch %s doctor #%d: %s", branch, i+1, strings.ReplaceAll(err.Error(), slug, "<slug>"))
}

// Crawl crawls the branches in order. Any failed page fails the whole crawl,
// and so does a branch listing no doctor (see Cgmh.crawlCampus).
func (s *Smile) Crawl(ctx context.Context) ([]Doctor, error) {
	taken := map[string]bool{}
	var doctors []Doctor
	for _, b := range s.branches {
		page, err := s.fetcher.Get(ctx, s.base+"/all-doctors/"+b.slug)
		if err != nil {
			return nil, fmt.Errorf("crawl: smile branch %s: %w", b.slug, err)
		}
		slugs := parseSmileBranch(page)
		if len(slugs) == 0 {
			return nil, fmt.Errorf("crawl: smile branch %s lists no doctors", b.slug)
		}
		for i, slug := range slugs {
			if taken[slug] {
				continue
			}
			taken[slug] = true
			u := s.base + "/doctors/" + slug
			profile, err := s.fetcher.Get(ctx, u)
			if err != nil {
				return nil, smileProfileError(b.slug, i, slug, err)
			}
			id, name, education, err := parseSmileProfile(profile)
			if err != nil {
				return nil, smileProfileError(b.slug, i, slug, err)
			}
			doctors = append(doctors, Doctor{
				Key:         "smile:" + id,
				Name:        name,
				City:        b.city,
				Hospital:    b.hospital,
				Departments: []string{"牙科"},
				Education:   education,
				URL:         u,
			})
		}
	}
	return doctors, nil
}
