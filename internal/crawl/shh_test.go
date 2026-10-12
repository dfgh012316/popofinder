package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// shhFixtureServer serves the synthetic fixtures by path and query; a page
// without a fixture is a 404, and failCode makes that department page a 500.
func shhFixtureServer(t *testing.T, failCode string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/page/team.aspx":
			serveFixture(w, "shh", "index.html")
		case "/page/TeamDoc.aspx":
			if q.Get("deptCode") == failCode {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			serveFixture(w, "shh", "dept_"+q.Get("deptCode")+".html")
		case "/page/TeamDocDetail.aspx":
			serveFixture(w, "shh", "profile_"+q.Get("docCode")+".html")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestParseShhIndex(t *testing.T) {
	got := parseShhIndex(readFixture(t, "shh", "index.html"))
	require.Equal(t, []shhDept{{"AB", "心臟內科"}, {"BD", "一般外科"}, {"01", "家庭醫學科"}}, got)
}

func TestParseShhDept(t *testing.T) {
	require.Equal(t, []string{"10001", "10002"}, parseShhDept(readFixture(t, "shh", "dept_AB.html")))
	require.Nil(t, parseShhDept(readFixture(t, "shh", "dept_01.html")))
}

func TestParseShhProfile(t *testing.T) {
	name, education, err := parseShhProfile(readFixture(t, "shh", "profile_10001.html"))
	require.NoError(t, err)
	require.Equal(t, "王小明", name)
	require.Equal(t, []string{"國立陽明大學醫學系", "國立台灣大學臨床醫學研究所碩士"}, education)

	name, education, err = parseShhProfile(readFixture(t, "shh", "profile_10003.html"))
	require.NoError(t, err)
	require.Equal(t, "張小美", name)
	require.Nil(t, education)

	_, _, err = parseShhProfile("<html></html>")
	require.Error(t, err)
}

func TestShhCrawlAggregatesDepartments(t *testing.T) {
	srv := shhFixtureServer(t, "")

	// The excluded 營養室 (15) has no fixture: fetching it would 404 and fail Crawl.
	got, err := NewShh(srv.URL, NewFetcher(0)).Crawl(context.Background())
	require.NoError(t, err)

	require.Len(t, got, 3)
	require.Equal(t, "shh:10001", got[0].Key)
	require.Equal(t, "shh:10002", got[1].Key)
	require.Equal(t, "shh:10003", got[2].Key)
	require.Equal(t, []string{"心臟內科", "一般外科"}, got[1].Departments)
	require.Equal(t, "李小華", got[1].Name)
	require.Equal(t, "新北", got[1].City)
	require.Equal(t, "雙和醫院", got[1].Hospital)
	require.Regexp(t, `/page/TeamDocDetail\.aspx\?deptCode=AB&docCode=10002$`, got[1].URL)
	require.Nil(t, got[2].Education)
}

func TestShhCrawlFailsOnMissingDeptPage(t *testing.T) {
	srv := shhFixtureServer(t, "BD")
	f := NewFetcher(0)
	f.sleep = func(time.Duration) {}

	got, err := NewShh(srv.URL, f).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}
