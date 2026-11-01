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
	// searchesPerMinute is how many search pages one IP may load per minute.
	searchesPerMinute = 30
	// lineAddURL adds the LINE bot as a friend.
	lineAddURL = "https://lin.ee/h1GxF9s"
)

//go:embed templates/*.html
var templateFS embed.FS

// pages holds one template set per page, each the layout plus that page's content.
var pages = func() map[string]*template.Template {
	m := map[string]*template.Template{}
	for _, name := range []string{"home", "results", "hospitals", "report", "message"} {
		m[name] = template.Must(template.ParseFS(templateFS, "templates/layout.html", "templates/"+name+".html"))
	}
	return m
}()

// store is what the pages read and write; *database.Repository satisfies it.
type store interface {
	Search(criteria search.Criteria, offset int) ([]database.MedicalPersonnel, database.SearchStats, error)
	LogSearch(ctx context.Context, criteria search.Criteria, resultCount int, channel string) error
	DirectoryHospitals(ctx context.Context) ([]database.DirectoryHospital, error)
}

var _ store = (*database.Repository)(nil)

// Handler serves the web pages.
type Handler struct {
	store   store
	limiter *limiter
}

// New returns the web pages backed by s.
func New(s store) *Handler {
	return &Handler{store: s, limiter: newLimiter(searchesPerMinute, time.Minute)}
}

// Register adds the web routes to r.
func (h *Handler) Register(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(securityHeaders)
		r.Get("/", h.home)
		r.Get("/search", h.search)
		r.Get("/hospitals", h.hospitals)
		r.Get("/report", h.report)
		r.Get("/robots.txt", robots)
	})
}

// page is what every page passes to the layout.
type page struct {
	Title   string
	Query   string // the search box value
	NoIndex bool
	LineURL string
	Content any
}

// example is a search shown on the home page, the same kinds as the help card's.
type example struct{ Query, Note string }

var examples = []example{
	{"陳", "姓名、醫院或科別含「陳」"},
	{"三總 牙科", "三總的牙科醫師"},
	{"台北 牙科", "台北的牙科醫師，縣市可以放在任何位置"},
	{"台北@醫院 三總", "只比對醫院名稱"},
	{"台北@科別 心臟", "只比對科別，打部分字即可"},
}

func (h *Handler) home(w http.ResponseWriter, r *http.Request) {
	render(w, http.StatusOK, "home", page{Content: struct {
		Coverage string
		Examples []example
		LineURL  string
	}{linetemplate.CoverageText, examples, lineAddURL}})
}

type resultCard struct {
	linetemplate.Card
	SameDepartmentLabel string
	SourceLinkLabel     string
}

type results struct {
	Query    string
	Summary  string
	Cards    []resultCard
	NoResult string
	PrevPage int // 0 when there is none
	NextPage int // 0 when there is none
	Capped   bool
	CappedAt int
}

func (h *Handler) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	p := page{Title: q, Query: q, NoIndex: true}
	if !h.limiter.allow(clientIP(r), time.Now()) {
		p.Content = "查詢太頻繁，請稍等一分鐘再試。"
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
	if pageNo == 1 {
		// Only the first page is a new search, as in LINE where paging is a postback.
		if err := h.store.LogSearch(r.Context(), criteria, stats.TotalCount, database.ChannelWeb); err != nil {
			slog.Error("log search", "err", err)
		}
	}

	res := results{Query: q, Summary: linetemplate.SearchSummary(criteria, stats), CappedAt: maxPages * database.PageSize}
	for _, person := range personnel {
		res.Cards = append(res.Cards, resultCard{
			Card:                linetemplate.NewCard(person),
			SameDepartmentLabel: linetemplate.SameDepartmentLabel,
			SourceLinkLabel:     linetemplate.SourceLinkLabel,
		})
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

type hospitalLink struct {
	Label, Query string
}

func (h *Handler) hospitals(w http.ResponseWriter, r *http.Request) {
	hs, err := h.store.DirectoryHospitals(r.Context())
	if err != nil {
		slog.Error("web hospitals", "err", err)
		render(w, http.StatusInternalServerError, "message", page{Content: "讀取失敗，請稍後再試。"})
		return
	}
	links := make([]hospitalLink, len(hs))
	for i, x := range hs {
		links[i] = hospitalLink{Label: x.Hospital + " " + strconv.Itoa(x.Count) + " 位", Query: linetemplate.HospitalSearchText(x)}
	}
	render(w, http.StatusOK, "hospitals", page{Title: "醫院清單", Content: struct {
		Links []hospitalLink
		Note  string
	}{links, linetemplate.HospitalListNote}})
}

func (h *Handler) report(w http.ResponseWriter, r *http.Request) {
	render(w, http.StatusOK, "report", page{Title: "補充、更正與下架", Content: struct {
		Text, Email string
	}{linetemplate.ReportText, linetemplate.ContactEmail}})
}

// robots keeps search results out of search engines (OD-17); the other pages have
// no doctor data and may be indexed.
func robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("User-agent: *\nDisallow: /search\n"))
}

func render(w http.ResponseWriter, status int, name string, p page) {
	p.LineURL = lineAddURL
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
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
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
