package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// femhFixtureServer serves the synthetic fixtures by path and query; a page
// without a fixture is a 404, and failCID makes that department page a 500.
func femhFixtureServer(t *testing.T, failCID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/section/section1.aspx":
			serveFixture(w, "femh", "index.html")
		case "/section/sectionDetail.aspx":
			if q.Get("CID") == failCID {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			serveFixture(w, "femh", "dept_"+q.Get("CID")+".html")
		case "/section/sectionDetail2.aspx":
			serveFixture(w, "femh", "profile_"+q.Get("DoctorID")+".html")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestParseFemhIndex(t *testing.T) {
	got := parseFemhIndex(readFixture(t, "femh", "index.html"))
	require.Equal(t, []femhDept{{"0401", "心臟血管內科"}, {"0203", "新陳代謝科"}}, got)
}

func TestParseFemhDept(t *testing.T) {
	require.Equal(t, []string{"10001", "10002"}, parseFemhDept(readFixture(t, "femh", "dept_0203.html")))
}

func TestParseFemhProfile(t *testing.T) {
	name, education, err := parseFemhProfile(readFixture(t, "femh", "profile_10001.html"))
	require.NoError(t, err)
	require.Equal(t, "王小明", name)
	require.Equal(t, []string{"輔仁大學公共衛生學系碩士", "國立台灣大學醫學系學士"}, education)

	name, education, err = parseFemhProfile(readFixture(t, "femh", "profile_10003.html"))
	require.NoError(t, err)
	require.Equal(t, "張小美", name)
	require.Nil(t, education)

	_, _, err = parseFemhProfile("<html><title>｜｜</title></html>")
	require.Error(t, err)
}

func TestFemhCrawlAggregatesDepartments(t *testing.T) {
	srv := femhFixtureServer(t, "")

	// The excluded 0534 unit has no fixture: fetching it would 404 and fail Crawl.
	got, err := NewFemh(srv.URL, NewFetcher(0)).Crawl(context.Background())
	require.NoError(t, err)

	require.Len(t, got, 3)
	require.Equal(t, "femh:10002", got[0].Key)
	require.Equal(t, "femh:10003", got[1].Key)
	require.Equal(t, "femh:10001", got[2].Key)
	require.Equal(t, []string{"心臟血管內科", "新陳代謝科"}, got[0].Departments)
	require.Equal(t, "新北", got[0].City)
	require.Equal(t, "亞東紀念醫院", got[0].Hospital)
	require.Equal(t, "李小華", got[0].Name)
	require.Regexp(t, `/section/sectionDetail2\.aspx\?CID=0401&&DoctorID=10002$`, got[0].URL)
}

func TestFemhCrawlFailsOnMissingDeptPage(t *testing.T) {
	srv := femhFixtureServer(t, "0203")
	f := NewFetcher(0)
	f.sleep = func(time.Duration) {}

	got, err := NewFemh(srv.URL, f).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}
