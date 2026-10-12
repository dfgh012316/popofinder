package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// cghFixtureServer serves the synthetic fixtures for every campus from one
// server, mapping the campus directory and page to "<campus>_<page>_<ids>.html";
// a page without a fixture (robots.txt included) is a 404. replace swaps one
// fixture name for another.
func cghFixtureServer(t *testing.T, replace map[string]string) *httptest.Server {
	t.Helper()
	campuses := map[string]string{
		"/ec99/rwd1320/":  "main",
		"/ec99/rwd102/":   "sijhih",
		"/ec99/rwd103/":   "hsinchu",
		"/ec99/rwd1309A/": "neihu",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dir, file := path.Split(r.URL.Path)
		campus, ok := campuses[dir]
		if !ok {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		var name string
		switch file {
		case "category.asp":
			name = campus + "_category_" + q.Get("category_id") + ".html"
		case "category_doct.asp":
			name = campus + "_list_" + q.Get("category_id") + "_" + q.Get("pageno") + ".html"
		case "product_doct.asp":
			name = campus + "_profile_" + q.Get("prodid") + ".html"
		default:
			http.NotFound(w, r)
			return
		}
		if to, ok := replace[name]; ok {
			name = to
		}
		serveFixture(w, "cgh", name)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newTestCgh crawls only the main and 汐止 campuses, all served by srv.
func newTestCgh(srv *httptest.Server, f *Fetcher) *Cgh {
	s := NewCgh(srv.URL, srv.URL, srv.URL, srv.URL, f)
	s.campuses = s.campuses[:2]
	return s
}

func TestParseCghIndex(t *testing.T) {
	got := parseCghIndex(readFixture(t, "cgh", "main_category_1553.html"))
	require.Equal(t, []cghDept{{"13", "消化內科"}, {"7", "心血管中心"}}, got)
}

func TestParseCghBranchIndex(t *testing.T) {
	got := parseCghBranchIndex(readFixture(t, "cgh", "sijhih_category_2.html"))
	require.Equal(t, []cghDept{{"58", "消化內科(原胃腸肝膽科)"}, {"61", "精神科"}}, got)
}

func TestParseCghListIDs(t *testing.T) {
	require.Equal(t, []string{"85"}, parseCghListIDs(readFixture(t, "cgh", "sijhih_category_58.html")))
	require.Nil(t, parseCghListIDs(readFixture(t, "cgh", "sijhih_category_61.html")))
}

func TestParseCghListPage(t *testing.T) {
	ids, last := parseCghListPage(readFixture(t, "cgh", "main_list_13_1.html"))
	require.Equal(t, []string{"20001", "20002"}, ids)
	require.Equal(t, 2, last)

	ids, last = parseCghListPage(readFixture(t, "cgh", "main_list_7_1.html"))
	require.Equal(t, []string{"20002"}, ids)
	require.Equal(t, 1, last)
}

func TestParseCghProfile(t *testing.T) {
	name, education, err := parseCghProfile(readFixture(t, "cgh", "main_profile_20001.html"))
	require.NoError(t, err)
	require.Equal(t, "王小明", name)
	require.Equal(t, []string{"國立台灣大學醫學系", "國立台灣大學臨床醫學研究所碩士"}, education)

	name, education, err = parseCghProfile(readFixture(t, "cgh", "main_profile_20003.html"))
	require.NoError(t, err)
	require.Equal(t, "張小美", name)
	require.Nil(t, education)

	_, _, err = parseCghProfile("<html></html>")
	require.Error(t, err)
}

func TestCghCrawlFirstCampusWins(t *testing.T) {
	srv := cghFixtureServer(t, nil)

	// These pages have no fixture, so fetching any of them would 404 and fail
	// Crawl: list 13 page 3, the off-site link 92, 營養組 60, and 汐止's profile of 20001.
	got, err := newTestCgh(srv, NewFetcher(0)).Crawl(context.Background())
	require.NoError(t, err)

	require.Len(t, got, 4)
	require.Equal(t, "cgh:20001", got[0].Key)
	require.Equal(t, "cgh:20002", got[1].Key)
	require.Equal(t, "cgh:20003", got[2].Key)
	require.Equal(t, "cgh:30001", got[3].Key)

	require.Equal(t, "國泰綜合醫院", got[0].Hospital)
	require.Equal(t, "台北", got[0].City)
	require.Equal(t, []string{"消化內科"}, got[0].Departments)
	require.Equal(t, "王小明", got[0].Name)

	require.Equal(t, []string{"消化內科", "心血管中心"}, got[1].Departments)

	require.Equal(t, "張小美", got[2].Name)
	require.Nil(t, got[2].Education)

	require.Equal(t, "汐止國泰綜合醫院", got[3].Hospital)
	require.Equal(t, "新北", got[3].City)
	require.Equal(t, []string{"消化內科(原胃腸肝膽科)"}, got[3].Departments)
	require.Regexp(t, `/ec99/rwd102/product_doct\.asp\?prodid=30001&category_id=85$`, got[3].URL)
}

func TestCghCrawlFailsWhenCampusHasNoDoctors(t *testing.T) {
	srv := cghFixtureServer(t, map[string]string{"sijhih_list_85_1.html": "list_empty.html"})
	f := NewFetcher(0)
	f.sleep = func(time.Duration) {}

	got, err := newTestCgh(srv, f).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}

func TestCghCrawlFailsOnMissingPage(t *testing.T) {
	srv := cghFixtureServer(t, map[string]string{"main_list_13_2.html": "missing.html"})
	f := NewFetcher(0)
	f.sleep = func(time.Duration) {}

	got, err := newTestCgh(srv, f).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}
