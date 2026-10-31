package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// kmshFixtureServer serves doc.asp as doc.html, doc_index.asp?dept_code=<c> as
// dept_<c>.html and search_data2.asp?pno=<n> as profile_<n>.html; replace swaps
// one fixture name for another.
func kmshFixtureServer(t *testing.T, replace map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var name string
		switch r.URL.Path {
		case "/pro/doc.asp":
			name = "doc.html"
		case "/pro/doc_index.asp":
			name = "dept_" + r.URL.Query().Get("dept_code") + ".html"
		case "/pro/search_data2.asp":
			name = "profile_" + r.URL.Query().Get("pno") + ".html"
		}
		if to, ok := replace[name]; ok {
			name = to
		}
		serveFixture(w, "kmsh", name)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestParseKmshDepts(t *testing.T) {
	// The second link's dept_code is followed by spaces, as on the real index.
	require.Equal(t, []kmshDept{{"0100", "內科"}, {"0200", "外科"}}, parseKmshDepts(readFixture(t, "kmsh", "doc.html")))
}

func TestParseKmshDoctors(t *testing.T) {
	pnos, path := parseKmshDoctors(readFixture(t, "kmsh", "dept_0100.html"))
	require.Equal(t, []string{"18", "25"}, pnos)
	require.Equal(t, "search_data2.asp?pno=18&dept_code=0100", path["18"])
}

func TestParseKmshProfile(t *testing.T) {
	name, education, err := parseKmshProfile(readFixture(t, "kmsh", "profile_18.html"))
	require.NoError(t, err)
	require.Equal(t, "王小明", name)
	require.Equal(t, []string{"高雄醫學大學醫學系", "高雄醫學大學臨床醫學研究所碩士"}, education)

	// Without a 學　歷 field the 經　歷 row is not read as education.
	name, education, err = parseKmshProfile(readFixture(t, "kmsh", "profile_25.html"))
	require.NoError(t, err)
	require.Equal(t, "李小華", name)
	require.Nil(t, education)

	_, _, err = parseKmshProfile("<html></html>")
	require.Error(t, err)
}

func TestKmshCrawl(t *testing.T) {
	srv := kmshFixtureServer(t, nil)

	got, err := NewKmsh(srv.URL+"/pro", NewFetcher(0)).Crawl(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, []string{"kmsh:18", "kmsh:25", "kmsh:31"}, []string{got[0].Key, got[1].Key, got[2].Key})
	require.Equal(t, "高雄市立小港醫院", got[0].Hospital)
	require.Equal(t, "高雄", got[0].City)
	require.Equal(t, srv.URL+"/pro/search_data2.asp?pno=18&dept_code=0100", got[0].URL)
	require.Equal(t, []string{"內科", "外科"}, got[1].Departments)
}

func TestKmshCrawlFailsWithoutDoctors(t *testing.T) {
	srv := kmshFixtureServer(t, map[string]string{"dept_0100.html": "dept_empty.html", "dept_0200.html": "dept_empty.html"})

	got, err := NewKmsh(srv.URL+"/pro", NewFetcher(0)).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}

func TestKmshCrawlFailsOnMissingPage(t *testing.T) {
	srv := kmshFixtureServer(t, map[string]string{"profile_31.html": "missing.html"})

	got, err := NewKmsh(srv.URL+"/pro", NewFetcher(0)).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}
