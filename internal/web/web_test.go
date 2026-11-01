package web

import (
	"context"
	"database/sql"
	"encoding/json"
	"html"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/search"
	linetemplate "github.com/dfgh012316/popofinder/internal/linebot/template"
)

type logged struct {
	criteria search.Criteria
	count    int
	channel  string
}

type fakeStore struct {
	personnel []database.MedicalPersonnel
	stats     database.SearchStats
	offsets   []int
	logs      []logged
	hospitals []database.DirectoryHospital
}

func (f *fakeStore) Search(c search.Criteria, offset int) ([]database.MedicalPersonnel, database.SearchStats, error) {
	f.offsets = append(f.offsets, offset)
	return f.personnel, f.stats, nil
}

func (f *fakeStore) LogSearch(_ context.Context, c search.Criteria, n int, channel string) error {
	f.logs = append(f.logs, logged{c, n, channel})
	return nil
}

func (f *fakeStore) DirectoryHospitals(context.Context) ([]database.DirectoryHospital, error) {
	return f.hospitals, nil
}

func serve(t *testing.T, h *Handler, target string, header ...string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	h.Register(r)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	for i := 0; i+1 < len(header); i += 2 {
		req.Header.Set(header[i], header[i+1])
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

type searchLink struct{ Q, Page, Text string }

var searchHref = regexp.MustCompile(`href="/search\?([^"]*)">([^<]*)<`)

// searchLinks decodes the /search links in body, whatever escaping the template chose.
func searchLinks(t *testing.T, body string) []searchLink {
	t.Helper()
	var links []searchLink
	for _, m := range searchHref.FindAllStringSubmatch(body, -1) {
		v, err := url.ParseQuery(html.UnescapeString(m[1]))
		require.NoError(t, err)
		links = append(links, searchLink{Q: v.Get("q"), Page: v.Get("page"), Text: m[2]})
	}
	return links
}

func doctor() database.MedicalPersonnel {
	return database.MedicalPersonnel{
		Name: "王<b>大明</b>", City: "桃園", Hospital: "林口長庚紀念醫院",
		Department:      sql.NullString{String: "心臟內科", Valid: true},
		Education:       sql.NullString{String: "某大學醫學系", Valid: true},
		Source:          sql.NullString{String: "hospital_directory", Valid: true},
		SourceURL:       sql.NullString{String: "https://hospital.example/doc/1", Valid: true},
		SourceCheckedAt: sql.NullTime{Time: time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), Valid: true},
	}
}

func TestHomeIsIndexable(t *testing.T) {
	rec := serve(t, New(&fakeStore{}), "/")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `action="/search"`)
	require.Contains(t, body, linetemplate.CoverageText)
	require.Contains(t, searchLinks(t, body), searchLink{Q: "台北@醫院 三總", Text: "台北@醫院 三總"})
	require.NotContains(t, body, "noindex")
	require.Empty(t, rec.Header().Get("X-Robots-Tag"))
	require.Equal(t, "no-referrer", rec.Header().Get("Referrer-Policy"))
	require.Contains(t, rec.Header().Get("Content-Security-Policy"), "default-src 'none'")
}

func TestSearchRendersCardsAndLogsFirstPage(t *testing.T) {
	f := &fakeStore{personnel: []database.MedicalPersonnel{doctor()}, stats: database.SearchStats{TotalCount: 15, CurrentPage: 1, TotalPages: 2, HasMore: true, MatchCounts: database.MatchCounts{Hospital: 15}}}
	rec := serve(t, New(f), "/search?q="+url.QueryEscape("林長 心臟"))
	require.Equal(t, http.StatusOK, rec.Code)

	// Same parsing, summary and card as LINE.
	criteria := search.ParseCriteria("林長 心臟")
	require.Equal(t, []int{0}, f.offsets)
	require.Equal(t, []logged{{criteria, 15, database.ChannelWeb}}, f.logs)
	body := rec.Body.String()
	require.Contains(t, body, linetemplate.SearchSummary(criteria, f.stats))
	require.Contains(t, body, "王&lt;b&gt;大明&lt;/b&gt;", "names are escaped")
	require.Contains(t, body, "點此查看來源（2026-10-31 查到）")
	require.Contains(t, body, `href="https://hospital.example/doc/1" target="_blank" rel="noopener noreferrer nofollow"`)
	links := searchLinks(t, body)
	require.Contains(t, links, searchLink{Q: linetemplate.NewCard(doctor()).SameDepartmentText, Text: "同院同科醫師"})
	require.Contains(t, links, searchLink{Q: "林長 心臟", Page: "2", Text: "下一頁 ›"})

	// Results stay out of search engines (OD-17).
	require.Equal(t, "noindex, nofollow", rec.Header().Get("X-Robots-Tag"))
	require.Contains(t, body, `<meta name="robots" content="noindex, nofollow">`)
}

func TestSearchLaterPagesAreNotLogged(t *testing.T) {
	f := &fakeStore{personnel: []database.MedicalPersonnel{doctor()}, stats: database.SearchStats{TotalCount: 25, CurrentPage: 2, TotalPages: 3, HasMore: true}}
	rec := serve(t, New(f), "/search?q=陳&page=2")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int{database.PageSize}, f.offsets)
	require.Empty(t, f.logs)
	require.Contains(t, searchLinks(t, rec.Body.String()), searchLink{Q: "陳", Page: "1", Text: "‹ 上一頁"})
}

func TestSearchPagingIsCapped(t *testing.T) {
	f := &fakeStore{personnel: []database.MedicalPersonnel{doctor()}, stats: database.SearchStats{TotalCount: 5000, CurrentPage: maxPages, TotalPages: 500, HasMore: true}}
	rec := serve(t, New(f), "/search?q=醫&page=999")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, []int{(maxPages - 1) * database.PageSize}, f.offsets)
	body := rec.Body.String()
	require.NotContains(t, body, "下一頁")
	require.Contains(t, body, "只顯示前 200 筆")
}

func TestSearchPastLastPageRedirects(t *testing.T) {
	f := &fakeStore{stats: database.SearchStats{TotalCount: 3, CurrentPage: 5, TotalPages: 1}}
	rec := serve(t, New(f), "/search?q=陳&page=5")
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "/search?q="+url.QueryEscape("陳"), rec.Header().Get("Location"))
}

func TestSearchNoResults(t *testing.T) {
	f := &fakeStore{stats: database.SearchStats{CurrentPage: 1}}
	rec := serve(t, New(f), "/search?q=不存在")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), linetemplate.NoResultsText)
	require.Len(t, f.logs, 1)
}

func TestSearchEmptyQueryRedirectsHome(t *testing.T) {
	rec := serve(t, New(&fakeStore{}), "/search?q=+")
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "/", rec.Header().Get("Location"))
}

func TestSearchTooLong(t *testing.T) {
	f := &fakeStore{}
	rec := serve(t, New(f), "/search?q="+strings.Repeat("陳", linetemplate.MaxMessageTextRunes+1))
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, f.offsets)
}

func TestSearchIsRateLimitedPerIP(t *testing.T) {
	f := &fakeStore{stats: database.SearchStats{CurrentPage: 1}}
	h := New(f)
	for i := 0; i < searchesPerMinute; i++ {
		require.Equal(t, http.StatusOK, serve(t, h, "/search?q=陳", "CF-Connecting-IP", "203.0.113.7").Code)
	}
	require.Equal(t, http.StatusTooManyRequests, serve(t, h, "/search?q=陳", "CF-Connecting-IP", "203.0.113.7").Code)
	require.Equal(t, http.StatusOK, serve(t, h, "/search?q=陳", "CF-Connecting-IP", "203.0.113.8").Code, "another IP is not affected")
	require.Len(t, f.offsets, searchesPerMinute+1)
}

// The server answers HEAD with the GET route (middleware.GetHead); a HEAD before a
// GET must not be logged as a second search.
func TestHeadSearchIsNotLogged(t *testing.T) {
	f := &fakeStore{stats: database.SearchStats{CurrentPage: 1}}
	rec := httptest.NewRecorder()
	New(f).search(rec, httptest.NewRequest(http.MethodHead, "/search?q=陳", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, f.logs)
}

func TestSearchIsLimitedPerDay(t *testing.T) {
	f := &fakeStore{stats: database.SearchStats{CurrentPage: 1}}
	h := New(f)
	h.perMinute = newLimiter(searchesPerDay+1, time.Minute) // only the daily limit applies
	for i := 0; i < searchesPerDay; i++ {
		require.Equal(t, http.StatusOK, serve(t, h, "/search?q=陳&page=2", "CF-Connecting-IP", "203.0.113.7").Code, "paging counts too")
	}
	rec := serve(t, h, "/search?q=陳", "CF-Connecting-IP", "203.0.113.7")
	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.Contains(t, rec.Body.String(), "今天的查詢次數已達上限")
	require.Equal(t, http.StatusOK, serve(t, h, "/search?q=陳", "CF-Connecting-IP", "203.0.113.8").Code, "another IP is not affected")
	require.Len(t, f.offsets, searchesPerDay+1)
}

func TestLimiterWindowResets(t *testing.T) {
	l := newLimiter(2, time.Minute)
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	require.True(t, l.allow("a", now))
	require.True(t, l.allow("a", now))
	require.False(t, l.allow("a", now.Add(59*time.Second)))
	require.True(t, l.allow("a", now.Add(time.Minute)))
}

func TestHospitalsLinksSearch(t *testing.T) {
	f := &fakeStore{hospitals: []database.DirectoryHospital{{City: "台北", Hospital: "馬偕紀念醫院", Count: 443}}}
	rec := serve(t, New(f), "/hospitals")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, searchLinks(t, body), searchLink{Q: linetemplate.HospitalSearchText(f.hospitals[0]), Text: "馬偕紀念醫院 443 位"})
	require.Contains(t, body, linetemplate.HospitalListNote)
}

func TestReportShowsContact(t *testing.T) {
	rec := serve(t, New(&fakeStore{}), "/report")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `href="mailto:`+linetemplate.ContactEmail+`"`)
	require.Contains(t, rec.Body.String(), "核對完就刪除")
}

// Every group must disallow /search: a crawler obeys only the group that names it
// most specifically, never also the * group (RFC 9309), so an AI group without it
// would open the search results (OD-17).
func TestRobotsEveryGroupDisallowsSearch(t *testing.T) {
	rec := serve(t, New(&fakeStore{}), "/robots.txt")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	body := rec.Body.String()

	var agents []string
	groups := 0
	for _, block := range strings.Split(body, "\n\n") {
		var lines []string
		for _, l := range strings.Split(block, "\n") {
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
				lines = append(lines, l)
			}
		}
		if len(lines) == 0 || !strings.HasPrefix(lines[0], "User-agent:") {
			continue
		}
		groups++
		for _, l := range lines {
			if a, ok := strings.CutPrefix(l, "User-agent: "); ok {
				agents = append(agents, a)
			}
		}
		require.Contains(t, lines, "Disallow: /search", "group %v", lines)
		for _, l := range lines {
			require.False(t, strings.HasPrefix(l, "Allow:"), "group %v must not allow anything back", lines)
		}
	}
	require.Equal(t, 2, groups)
	for _, a := range []string{"*", "GPTBot", "OAI-SearchBot", "ChatGPT-User", "ClaudeBot", "PerplexityBot", "Google-Extended", "CCBot"} {
		require.Contains(t, agents, a)
	}
	require.Contains(t, body, "Sitemap: "+siteURL+"/sitemap.xml")
}

func TestHomeSEO(t *testing.T) {
	body := serve(t, New(&fakeStore{}), "/").Body.String()
	require.Contains(t, body, `<link rel="canonical" href="`+siteURL+`/">`)
	require.Contains(t, body, `<meta property="og:image" content="`+siteURL+`/og.png">`)
	require.Contains(t, body, `<meta name="description" content="`+siteDescription+`">`)
	require.Contains(t, body, `<link rel="icon" href="/favicon.svg"`)
	for _, d := range differences {
		require.Contains(t, body, d.Title)
	}

	// The JSON-LD is valid and says what the page shows.
	_, rest, ok := strings.Cut(body, `<script type="application/ld+json">`)
	require.True(t, ok)
	raw, _, ok := strings.Cut(rest, "</script>")
	require.True(t, ok)
	var ld []struct {
		Type        string   `json:"@type"`
		FeatureList []string `json:"featureList"`
		MainEntity  []struct {
			Name   string `json:"name"`
			Answer struct {
				Text string `json:"text"`
			} `json:"acceptedAnswer"`
		} `json:"mainEntity"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &ld))
	require.Len(t, ld, 2)
	require.Equal(t, "WebApplication", ld[0].Type)
	require.Len(t, ld[0].FeatureList, len(differences))
	require.Equal(t, "FAQPage", ld[1].Type)
	require.Len(t, ld[1].MainEntity, len(faqs))
	for i, f := range faqs {
		require.Equal(t, f.Q, ld[1].MainEntity[i].Name)
		require.Equal(t, f.A, ld[1].MainEntity[i].Answer.Text)
		require.Contains(t, body, f.Q, "the FAQ is visible on the page too")
	}
}

func TestSearchHasNoCanonicalOrJSONLD(t *testing.T) {
	f := &fakeStore{stats: database.SearchStats{CurrentPage: 1}}
	body := serve(t, New(f), "/search?q=陳").Body.String()
	require.NotContains(t, body, `rel="canonical"`)
	require.NotContains(t, body, "application/ld+json")
}

func TestLLMsTxt(t *testing.T) {
	rec := serve(t, New(&fakeStore{}), "/llms.txt")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "text/plain; charset=utf-8", rec.Header().Get("Content-Type"))
	body := rec.Body.String()
	require.True(t, strings.HasPrefix(body, "# POPO Finder"))
	for _, d := range differences {
		require.Contains(t, body, "**"+d.Title+"**：")
	}
	for _, f := range faqs {
		require.Contains(t, body, "### "+f.Q)
	}
	require.Contains(t, body, linetemplate.ContactEmail)
	require.NotContains(t, body, "<no value>")
}

func TestSitemapListsOnlyIndexablePages(t *testing.T) {
	rec := serve(t, New(&fakeStore{}), "/sitemap.xml")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, p := range []string{"/", "/hospitals", "/report"} {
		require.Contains(t, body, "<loc>"+siteURL+p+"</loc>")
	}
	require.NotContains(t, body, "<loc>"+siteURL+"/search")
}

func TestStaticAssets(t *testing.T) {
	rec := serve(t, New(&fakeStore{}), "/og.png")
	require.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	img, err := png.DecodeConfig(rec.Body)
	require.NoError(t, err)
	require.Equal(t, [2]int{1200, 630}, [2]int{img.Width, img.Height}, "Open Graph size")

	rec = serve(t, New(&fakeStore{}), "/favicon.svg")
	require.Equal(t, "image/svg+xml", rec.Header().Get("Content-Type"))
	require.Contains(t, rec.Body.String(), "<svg")
}
