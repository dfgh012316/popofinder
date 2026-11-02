// Package web serves the POPO Finder web version (docs/open-decisions.md OD-17): the
// same search, doctor cards and texts as the LINE bot, rendered as HTML by this binary.
package web

import (
	"bytes"
	"context"
	"embed"
	"html/template"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/search"
	linetemplate "github.com/dfgh012316/popofinder/internal/linebot/template"
)

const (
	// maxPages caps how far one search can page (200 results): wide searches have
	// to be narrowed, and the whole database cannot be paged through (OD-17).
	maxPages = 20
	// searchesPerMinute and searchesPerDay are how many search pages one IP may load
	// per minute and per day. Copying the database takes about 1,300 page loads
	// (OD-17); people search a few times a day.
	searchesPerMinute = 30
	searchesPerDay    = 100
	// lineAddURL adds the LINE bot as a friend.
	lineAddURL = "https://lin.ee/h1GxF9s"
)

//go:embed templates/*.html
var templateFS embed.FS

// funcs are the template helpers: numbers with thousands separators and the
// coverage bars' widths.
var funcs = template.FuncMap{
	"comma": func(n int) string {
		s := strconv.Itoa(n)
		for i := len(s) - 3; i > 0; i -= 3 {
			s = s[:i] + "," + s[i:]
		}
		return s
	},
	"minus": func(a, b int) int { return a - b },
	// maxCity is the largest city total, the full width of a bar.
	"maxCity": func(c *coverage) int {
		m := 0
		for _, r := range c.Regions {
			for _, x := range r.Cities {
				m = max(m, x.Total)
			}
		}
		return m
	},
	"pct": func(n, of int) string {
		if of <= 0 {
			return "0%"
		}
		return strconv.FormatFloat(float64(n)*100/float64(of), 'f', 2, 64) + "%"
	},
}

// pages holds one template set per page, each the layout plus that page's content.
var pages = func() map[string]*template.Template {
	m := map[string]*template.Template{}
	for _, name := range []string{"home", "results", "hospitals", "report", "message"} {
		m[name] = template.Must(template.New(name).Funcs(funcs).ParseFS(templateFS, "templates/layout.html", "templates/"+name+".html"))
	}
	return m
}()

// store is what the pages read and write; *database.Repository satisfies it.
type store interface {
	Search(criteria search.Criteria, offset int) ([]database.MedicalPersonnel, database.SearchStats, error)
	LogSearch(ctx context.Context, criteria search.Criteria, resultCount int, channel string) error
	SearchFacets(ctx context.Context, criteria search.Criteria) ([]database.Facet, error)
	DirectoryHospitals(ctx context.Context) ([]database.DirectoryHospital, error)
	CityCounts(ctx context.Context) ([]database.CityCount, error)
}

var _ store = (*database.Repository)(nil)

// Handler serves the web pages.
type Handler struct {
	store     store
	perMinute *limiter
	perDay    *limiter
}

// New returns the web pages backed by s.
func New(s store) *Handler {
	return &Handler{
		store:     s,
		perMinute: newLimiter(searchesPerMinute, time.Minute),
		perDay:    newLimiter(searchesPerDay, 24*time.Hour),
	}
}

// Register adds the web routes to r.
func (h *Handler) Register(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(securityHeaders)
		r.Get("/", h.home)
		r.Get("/search", h.search)
		r.Get("/hospitals", h.hospitals)
		r.Get("/report", h.report)
		r.Get("/robots.txt", staticFile("robots.txt", "text/plain; charset=utf-8"))
		r.Get("/sitemap.xml", staticFile("sitemap.xml", "application/xml; charset=utf-8"))
		r.Get("/llms.txt", serveBytes(llmsTxt, "text/plain; charset=utf-8"))
		r.Get("/og.png", staticFile("og.png", "image/png"))
		r.Get("/favicon.svg", staticFile("favicon.svg", "image/svg+xml"))
	})
}

// page is what every page passes to the layout.
type page struct {
	Title       string // "" on the home page
	Description string // "" means siteDescription
	Canonical   string // full URL of a page that may be indexed; "" for search results
	Query       string // the search box value
	NoIndex     bool
	JSONLD      template.JS
	Content     any

	Nav string // the header link of this page: "hospitals", "report" or ""

	// Set by render.
	HeadTitle, OGImage, LineURL, RepoURL string
	HeaderSearch                         bool // every page but home has the search box in the header
}

// example is a search shown on the home page, the same kinds as the help card's.
type example struct{ Query, Note string }

var examples = []example{
	{"三總 牙科", "醫院簡稱加科別"},
	{"林長 心臟", "林口長庚的心臟科"},
	{"高雄 皮膚", "縣市加科別"},
	{"陳", "姓名、醫院或科別含「陳」"},
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	cov, hs, err := h.loadCoverage(r.Context())
	if err != nil {
		// The search form still works without the numbers.
		slog.Error("web home coverage", "err", err)
	}
	names := make([]string, len(hs))
	for i, x := range hs {
		names[i] = x.Hospital
	}
	render(w, http.StatusOK, "home", page{Canonical: siteURL + "/", JSONLD: homeJSONLD, Content: struct {
		Coverage     string
		Examples     []example
		Regions      []region
		HospitalList []string
		Stats        *coverage
		LineURL      string
		Differences  []point
		FAQs         any
	}{linetemplate.CoverageText, examples, regions, names, cov, lineAddURL, differences, faqs}})
}

type resultCard struct {
	linetemplate.Card
	EducationLines      []string
	EducationLabel      string
	SourceLabel         string
	SameDepartmentLabel string
	SourceLinkLabel     string
}

type results struct {
	Query       string
	Heading     string   // the summary's first line
	SummaryRest []string // its other lines
	Cards       []resultCard
	NoResult    string
	Facets      facets
	PrevPage    int // 0 when there is none
	NextPage    int // 0 when there is none
	Page        int
	TotalPages  int
	Capped      bool
	CappedAt    int
}

// facetLink adds words to the current search.
type facetLink struct {
	Label, Note, Query string
	Count              int
}

type facets struct {
	Cities, Hospitals []facetLink
}

// maxHospitalFacets is how many hospitals a wide search offers to narrow to.
const maxHospitalFacets = 8

// buildFacets offers the cities and hospitals of a wide search's matches as words
// to add. Only for keyword searches: in an @醫院 or @科別 search an added word is
// read as another hospital or department, not a city. A facet that would not
// narrow the search (a single city or hospital) is left out.
func buildFacets(q string, criteria search.Criteria, fs []database.Facet) facets {
	var out facets
	if criteria.SearchType != search.TypeAll {
		return out
	}
	byCity := map[string]int{}
	var cities []string
	for _, f := range fs {
		if _, ok := byCity[f.City]; !ok {
			cities = append(cities, f.City)
		}
		byCity[f.City] += f.Count
	}
	if criteria.City == nil && len(cities) > 1 {
		for _, c := range cities {
			out.Cities = append(out.Cities, facetLink{Label: c, Query: q + " " + c, Count: byCity[c]})
		}
		slices.SortStableFunc(out.Cities, func(a, b facetLink) int { return b.Count - a.Count })
	}
	if len(fs) > 1 {
		for _, f := range fs[:min(len(fs), maxHospitalFacets)] {
			// The city too, unless the search has one: 國泰綜合醫院 is also part of
			// 汐止國泰綜合醫院. A second city word would be read as a keyword.
			add := f.City + " " + f.Hospital
			if criteria.City != nil {
				add = f.Hospital
			}
			out.Hospitals = append(out.Hospitals, facetLink{Label: f.Hospital, Note: f.City, Query: q + " " + add, Count: f.Count})
		}
	}
	return out
}

// searchParams are the optional fields of the home page's search form.
var searchParams = []string{"hospital", "dept", "city"}

// composeQuery joins the search form's fields into one search text, as a user would
// type it; ok is false when the form sent no separate field. The city field is left
// out when the typed text already names a city: ParseCriteria takes only the first
// city word and would read a second one as a keyword every result must contain.
// The results heading shows which city was searched.
func composeQuery(v url.Values) (q string, ok bool) {
	typed := strings.TrimSpace(v.Get("q"))
	words := []string{typed}
	for _, k := range searchParams {
		x := strings.TrimSpace(v.Get(k))
		if x == "" {
			continue
		}
		ok = true
		if k == "city" && typed != "" && search.ParseCriteria(typed).City != nil {
			continue
		}
		words = append(words, x)
	}
	return strings.TrimSpace(strings.Join(words, " ")), ok
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q, composed := composeQuery(r.URL.Query())
	if q == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if composed {
		// One search text, so paging, facets and the log work as for a typed search.
		http.Redirect(w, r, "/search?q="+url.QueryEscape(q), http.StatusSeeOther)
		return
	}
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	p := page{Title: q, Query: q, NoIndex: true}
	ip, now := clientIP(r), time.Now()
	if !h.perMinute.allow(ip, now) {
		p.Content = "查詢太頻繁，請稍等一分鐘再試。"
		render(w, http.StatusTooManyRequests, "message", p)
		return
	}
	if !h.perDay.allow(ip, now) {
		p.Content = "今天的查詢次數已達上限，請明天再試。"
		render(w, http.StatusTooManyRequests, "message", p)
		return
	}
	if utf8.RuneCountInString(q) > linetemplate.MaxMessageTextRunes {
		p.Content = "查詢太長，請縮短後再試。"
		render(w, http.StatusBadRequest, "message", p)
		return
	}

	pageNo, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageNo = min(max(pageNo, 1), maxPages)
	criteria := search.ParseCriteria(q)
	personnel, stats, err := h.store.Search(criteria, (pageNo-1)*database.PageSize)
	if err != nil {
		slog.Error("web search", "err", err)
		p.Content = "查詢失敗，請稍後再試。"
		render(w, http.StatusInternalServerError, "message", p)
		return
	}
	if len(personnel) == 0 && stats.TotalCount > 0 {
		// A page past the last one.
		http.Redirect(w, r, "/search?q="+url.QueryEscape(q), http.StatusSeeOther)
		return
	}
	if pageNo == 1 && r.Method == http.MethodGet {
		// Only the first page is a new search, as in LINE where paging is a postback.
		// A HEAD (a link checker before its GET) is not a search.
		if err := h.store.LogSearch(r.Context(), criteria, stats.TotalCount, database.ChannelWeb); err != nil {
			slog.Error("log search", "err", err)
		}
	}

	summary := strings.Split(linetemplate.SearchSummary(criteria, stats), "\n")
	res := results{Query: q, Heading: summary[0], SummaryRest: summary[1:], Page: pageNo,
		TotalPages: min(stats.TotalPages, maxPages), CappedAt: maxPages * database.PageSize}
	for _, person := range personnel {
		card := linetemplate.NewCard(person)
		res.Cards = append(res.Cards, resultCard{
			Card:                card,
			EducationLines:      strings.Split(card.Education, "；"),
			EducationLabel:      linetemplate.EducationLabel,
			SourceLabel:         linetemplate.SourceLabel,
			SameDepartmentLabel: linetemplate.SameDepartmentLabel,
			SourceLinkLabel:     linetemplate.SourceLinkLabel,
		})
	}
	if stats.TotalCount > database.PageSize {
		fs, err := h.store.SearchFacets(r.Context(), criteria)
		if err != nil {
			slog.Error("web search facets", "err", err)
		}
		res.Facets = buildFacets(q, criteria, fs)
	}
	if len(personnel) == 0 {
		res.NoResult = linetemplate.NoResultsText
	}
	if pageNo > 1 {
		res.PrevPage = pageNo - 1
	}
	if stats.HasMore {
		if pageNo < maxPages {
			res.NextPage = pageNo + 1
		} else {
			res.Capped = true
		}
	}
	p.Content = res
	render(w, http.StatusOK, "results", p)
}

// region groups cities as the 國發會 does; every search city is in one region.
type region struct {
	Name, ID string
	Cities   []string
}

var regions = []region{
	{"北部", "north", []string{"台北", "新北", "基隆", "桃園", "新竹", "宜蘭"}},
	{"中部", "central", []string{"苗栗", "台中", "彰化", "南投", "雲林"}},
	{"南部", "south", []string{"嘉義", "台南", "高雄", "屏東"}},
	{"東部", "east", []string{"花蓮", "台東"}},
}

type coverageHospital struct {
	Name, Query, CheckedOn string
	Count                  int
}

type coverageCity struct {
	City                string
	Total, Official     int
	Hospitals           []coverageHospital
	OfficialHospitalCnt int
}

type coverageRegion struct {
	Name, ID string
	Total    int
	Cities   []coverageCity
}

// coverage is what the home page and /hospitals say is listed.
type coverage struct {
	Total, Official, Hospitals int
	CheckedOn                  string // the latest date any official directory was read
	Regions                    []coverageRegion
}

// loadCoverage counts the listed doctors by region and city, with each city's
// hospitals whose official directories are crawled.
func (h *Handler) loadCoverage(ctx context.Context) (*coverage, []database.DirectoryHospital, error) {
	cs, err := h.store.CityCounts(ctx)
	if err != nil {
		return nil, nil, err
	}
	hs, err := h.store.DirectoryHospitals(ctx)
	if err != nil {
		return nil, nil, err
	}
	counts := map[string]database.CityCount{}
	cov := &coverage{Hospitals: len(hs)}
	for _, c := range cs {
		counts[c.City] = c
		cov.Total += c.Total
		cov.Official += c.Official
	}
	byCity := map[string][]coverageHospital{}
	for _, x := range hs {
		checked := ""
		if x.CheckedAt.Valid {
			checked = x.CheckedAt.Time.Format("2006-01-02")
			cov.CheckedOn = max(cov.CheckedOn, checked)
		}
		byCity[x.City] = append(byCity[x.City], coverageHospital{
			Name: x.Hospital, Query: linetemplate.HospitalSearchText(x), CheckedOn: checked, Count: x.Count,
		})
	}
	for _, g := range regions {
		cr := coverageRegion{Name: g.Name, ID: g.ID}
		for _, city := range g.Cities {
			c := counts[city]
			cr.Total += c.Total
			cr.Cities = append(cr.Cities, coverageCity{City: city, Total: c.Total, Official: c.Official,
				Hospitals: byCity[city], OfficialHospitalCnt: len(byCity[city])})
		}
		cov.Regions = append(cov.Regions, cr)
	}
	return cov, hs, nil
}

func (h *Handler) hospitals(w http.ResponseWriter, r *http.Request) {
	cov, _, err := h.loadCoverage(r.Context())
	if err != nil {
		slog.Error("web hospitals", "err", err)
		render(w, http.StatusInternalServerError, "message", page{Content: "讀取失敗，請稍後再試。"})
		return
	}
	render(w, http.StatusOK, "hospitals", page{Title: "收錄範圍", Nav: "hospitals", Canonical: siteURL + "/hospitals",
		Description: "各縣市收錄的醫師人數，以及已收錄官網完整醫師名單的醫院與查到日期。", Content: struct {
			Stats *coverage
			Note  string
		}{cov, linetemplate.HospitalListNote}})
}

func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	render(w, http.StatusOK, "report", page{Title: "補充、更正與下架", Nav: "report", Canonical: siteURL + "/report",
		Description: "醫師或院所補充學歷、更正資料或要求下架的方式。", Content: struct {
			Paragraphs            []string
			Email, AddURL, FixURL string
		}{strings.Split(linetemplate.ReportText, "\n\n"), linetemplate.ContactEmail,
			mailto("補充醫師學歷"), mailto("更正或下架")}})
}

// mailto is a link that writes to the contact address with subject filled in.
func mailto(subject string) string {
	return "mailto:" + linetemplate.ContactEmail + "?subject=" + url.PathEscape(subject)
}

func render(w http.ResponseWriter, status int, name string, p page) {
	p.LineURL = lineAddURL
	p.RepoURL = repoURL
	p.HeaderSearch = name != "home"
	p.OGImage = siteURL + "/og.png"
	if p.Description == "" {
		p.Description = siteDescription
	}
	p.HeadTitle = "POPO Finder 醫師學歷查詢｜查醫師在哪裡受醫學教育"
	if p.Title != "" {
		p.HeadTitle = p.Title + " - POPO Finder 醫師學歷查詢"
	}
	var buf bytes.Buffer
	if err := pages[name].ExecuteTemplate(&buf, "layout", p); err != nil {
		slog.Error("web render", "page", name, "err", err)
		http.Error(w, "頁面產生失敗", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// securityHeaders: no scripts or outside resources, and no Referer, so a hospital
// site opened from a card does not learn what was searched.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// clientIP is the visitor's address. Requests arrive through the Cloudflare tunnel,
// which sets CF-Connecting-IP; RemoteAddr is the tunnel itself.
func clientIP(r *http.Request) string {
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
