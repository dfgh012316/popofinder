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
	facets    []database.Facet
	cities    []database.CityCount
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

func (f *fakeStore) SearchFacets(context.Context, search.Criteria) ([]database.Facet, error) {
	return f.facets, nil
}

func (f *fakeStore) CityCounts(context.Context) ([]database.CityCount, error) {
	return f.cities, nil
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

var searchHref = regexp.MustCompile(`href="/search\?([^"]*)"[^>]*>([^<]*)<`)

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
	require.Contains(t, searchLinks(t, body), searchLink{Q: "三總 牙科", Text: "三總 牙科"})
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
	summary := strings.Split(linetemplate.SearchSummary(criteria, f.stats), "\n")
	require.Contains(t, body, "<h1>"+summary[0]+"</h1>", "the summary's first line is the heading")
	for _, line := range summary[1:] {
		require.Contains(t, body, "<span>"+line+"</span>")
	}
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
	f := &fakeStore{
		cities:    []database.CityCount{{City: "台北", Total: 500, Official: 443}, {City: "台中", Total: 20}},
		hospitals: []database.DirectoryHospital{{City: "台北", Hospital: "馬偕紀念醫院", Count: 443, CheckedAt: sql.NullTime{Time: time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC), Valid: true}}},
	}
	rec := serve(t, New(f), "/hospitals")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, searchLinks(t, body), searchLink{Q: linetemplate.HospitalSearchText(f.hospitals[0]), Text: "馬偕紀念醫院"})
	require.Contains(t, body, "443 位，2026-10-12 查到")
	require.Contains(t, body, "共 <b class=\"num\">520</b> 位醫師，其中 <b class=\"num\">443</b> 位來自 1 家醫療院所官網的完整名單（最近一次查到 <span class=\"num\">2026-10-12</span>）")
	// A city without an official directory says where its doctors come from.
	require.Contains(t, searchLinks(t, body), searchLink{Q: "台中", Text: "查台中的醫師"})
	require.Contains(t, body, linetemplate.HospitalListNote)
}

func TestHomeShowsCoverage(t *testing.T) {
	f := &fakeStore{
		cities:    []database.CityCount{{City: "台北", Total: 1200, Official: 900}, {City: "高雄", Total: 300}},
		hospitals: []database.DirectoryHospital{{City: "台北", Hospital: "馬偕紀念醫院", Count: 900}},
	}
	rec := serve(t, New(f), "/")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, "共 <b class=\"num\">1,500</b> 位醫師，其中 <b class=\"num\">900</b> 位來自 1 家醫療院所官網的完整名單")
	links := searchLinks(t, body)
	require.Contains(t, links, searchLink{Q: "台北", Text: "台北"})
	require.Contains(t, links, searchLink{Q: "高雄", Text: "高雄"})
	require.NotContains(t, links, searchLink{Q: "台東", Text: "台東"}, "a city with no doctor is not a link")
	// The bars are shares of the largest city: 台北 900 + 300 of 1,200, 高雄 300.
	require.Contains(t, body, `<span class="o" style="width:75.00%"></span><span class="x" style="width:25.00%"></span>`)
	require.Contains(t, body, `<span class="x" style="width:25.00%"></span></span>
<span class="num">300</span>`)
	require.Contains(t, body, "官網名單 1 家")
	require.Contains(t, body, `<option value="馬偕紀念醫院">`, "the hospital field suggests the listed hospitals")
}

func TestHomeWorksWithoutCoverage(t *testing.T) {
	rec := serve(t, New(&failingCoverage{}), "/")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `action="/search"`)
	require.NotContains(t, rec.Body.String(), `id="coverage"`)
}

type failingCoverage struct{ fakeStore }

func (*failingCoverage) CityCounts(context.Context) ([]database.CityCount, error) {
	return nil, sql.ErrConnDone
}

func TestRegionsCoverEveryCity(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range regions {
		for _, c := range r.Cities {
			require.False(t, seen[c], "%s is in two regions", c)
			seen[c] = true
			got := search.ParseCriteria(c).City
			require.NotNil(t, got, "%s is a search city", c)
			require.Equal(t, c, *got)
		}
	}
	require.Len(t, seen, 17)
}

func TestSearchComposesFormFields(t *testing.T) {
	f := &fakeStore{}
	rec := serve(t, New(f), "/search?q=王&city=台中&hospital=中榮&dept=心臟")
	require.Equal(t, http.StatusSeeOther, rec.Code)
	loc, err := url.Parse(rec.Header().Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "/search", loc.Path)
	require.Equal(t, "台中 王 中榮 心臟", loc.Query().Get("q"))
	require.Empty(t, f.offsets, "the redirect does not search")

	// Empty fields, as the form sends them, change nothing.
	rec = serve(t, New(f), "/search?q=陳&city=&hospital=&dept=")
	require.Equal(t, http.StatusOK, rec.Code)
	require.Len(t, f.offsets, 1)

	rec = serve(t, New(f), "/search?q=&city=&hospital=&dept=")
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "/", rec.Header().Get("Location"))
}

// The composed text must mean what the form said once ParseCriteria reads it.
func TestComposeQueryKeepsTheFormsMeaning(t *testing.T) {
	city := func(c string) *string { return &c }
	for _, tc := range []struct {
		form string
		want string
		is   search.Criteria
	}{
		{"q=王&city=台北&dept=心臟", "台北 王 心臟", search.Criteria{SearchType: search.TypeAll, SearchTerm: "王 心臟", City: city("台北")}},
		{"q=&city=台中&dept=心臟", "台中 心臟", search.Criteria{SearchType: search.TypeAll, SearchTerm: "心臟", City: city("台中")}},
		// A city typed in the box is not added again, and wins over the field.
		{"q=台北%20王&city=台北", "台北 王", search.Criteria{SearchType: search.TypeAll, SearchTerm: "王", City: city("台北")}},
		{"q=王%20台北市&city=台北", "台北 王", search.Criteria{SearchType: search.TypeAll, SearchTerm: "王", City: city("台北")}},
		{"q=台北%20王&city=高雄", "台北 王", search.Criteria{SearchType: search.TypeAll, SearchTerm: "王", City: city("台北")}},
		// An @ search keeps its mode with only a city, the city first.
		{"q=@醫院%20三總&city=台北", "台北 @醫院 三總", search.Criteria{SearchType: search.TypeHospital, SearchTerm: "三總", City: city("台北")}},
		{"q=@科別%20心臟&city=台北", "台北 @科別 心臟", search.Criteria{SearchType: search.TypeDepartment, SearchTerm: "心臟", City: city("台北")}},
		{"q=台北@醫院%20三總&city=高雄", "台北 @醫院 三總", search.Criteria{SearchType: search.TypeHospital, SearchTerm: "三總", City: city("台北")}},
		// With a hospital or department field it becomes a keyword search.
		{"q=@醫院%20三總&dept=心臟", "三總 心臟", search.Criteria{SearchType: search.TypeAll, SearchTerm: "三總 心臟"}},
		// A first word that is a hospital alias starting with a city is kept whole.
		{"q=台中榮總&city=台中&dept=心臟", "台中 台中榮總 心臟", search.Criteria{SearchType: search.TypeAll, SearchTerm: "台中榮總 心臟", City: city("台中")}},
	} {
		v, err := url.ParseQuery(tc.form)
		require.NoError(t, err)
		got, ok := composeQuery(v)
		require.True(t, ok, tc.form)
		require.Equal(t, tc.want, got, tc.form)
		require.Equal(t, tc.is, search.ParseCriteria(got), tc.form)
	}
	got, ok := composeQuery(url.Values{"q": {"@醫院 三總"}})
	require.False(t, ok, "no separate field: the typed text as it is")
	require.Equal(t, "@醫院 三總", got)
}

func TestSearchOffersFacets(t *testing.T) {
	f := &fakeStore{
		personnel: []database.MedicalPersonnel{doctor()},
		stats:     database.SearchStats{TotalCount: 30, CurrentPage: 1, TotalPages: 3, HasMore: true, MatchCounts: database.MatchCounts{Name: 30}},
		facets:    []database.Facet{{City: "台北", Hospital: "馬偕紀念醫院", Count: 12}, {City: "新北", Hospital: "亞東紀念醫院", Count: 10}, {City: "台北", Hospital: "國泰綜合醫院", Count: 8}},
	}
	links := searchLinks(t, serve(t, New(f), "/search?q=陳").Body.String())
	require.Contains(t, links, searchLink{Q: "陳 台北", Text: "台北"})
	require.Contains(t, links, searchLink{Q: "陳 新北", Text: "新北"})
	require.Contains(t, links, searchLink{Q: "陳 台北 馬偕紀念醫院", Text: "馬偕紀念醫院"}, "the city keeps 國泰綜合醫院 from also matching 汐止國泰綜合醫院")
}

func TestBuildFacets(t *testing.T) {
	fs := []database.Facet{{City: "台北", Hospital: "馬偕紀念醫院", Count: 5}, {City: "新北", Hospital: "亞東紀念醫院", Count: 7}, {City: "新北", Hospital: "雙和醫院", Count: 1}}

	got := buildFacets("陳", search.ParseCriteria("陳"), fs)
	require.Equal(t, []facetLink{{Label: "新北", Query: "陳 新北", Count: 8}, {Label: "台北", Query: "陳 台北", Count: 5}}, got.Cities, "most first")
	require.Len(t, got.Hospitals, 3)

	// With a city, no city facets, and a hospital facet adds no second city word.
	got = buildFacets("台北 陳", search.ParseCriteria("台北 陳"), fs[:1])
	require.Empty(t, got.Cities)
	require.Empty(t, got.Hospitals, "a single hospital does not narrow")
	got = buildFacets("台北 陳", search.ParseCriteria("台北 陳"), []database.Facet{fs[0], {City: "台北", Hospital: "國泰綜合醫院", Count: 2}})
	require.Equal(t, "台北 陳 馬偕紀念醫院", got.Hospitals[0].Query)

	// In an @醫院 search an added word would be read as another hospital.
	require.Equal(t, facets{}, buildFacets("台北@醫院 三總", search.ParseCriteria("台北@醫院 三總"), fs))
}

func TestReportShowsContact(t *testing.T) {
	rec := serve(t, New(&fakeStore{}), "/report")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	require.Contains(t, body, `href="mailto:`+linetemplate.ContactEmail+`?subject=%E8%A3%9C%E5%85%85%E9%86%AB%E5%B8%AB%E5%AD%B8%E6%AD%B7"`, "the subject 補充醫師學歷, encoded")
	require.Contains(t, body, linetemplate.ContactEmail)
	require.Contains(t, body, "核對完就刪除")
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
