package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func readFemhFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "femh", name))
	require.NoError(t, err)
	return string(b)
}

// femhFixtureServer serves the synthetic fixtures by path and query; a page
// without a fixture is a 404, and failCID makes that department page a 500.
func femhFixtureServer(t *testing.T, failCID string) *httptest.Server {
	t.Helper()
	serve := func(w http.ResponseWriter, name string) {
		b, err := os.ReadFile(filepath.Join("testdata", "femh", name))
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write(b)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/section/section1.aspx":
			serve(w, "index.html")
		case "/section/sectionDetail.aspx":
			if q.Get("CID") == failCID {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			serve(w, "dept_"+q.Get("CID")+".html")
		case "/section/sectionDetail2.aspx":
			serve(w, "profile_"+q.Get("DoctorID")+".html")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestParseFemhIndex(t *testing.T) {
	got := parseFemhIndex(readFemhFixture(t, "index.html"))
	require.Equal(t, []femhDept{{"0401", "心臟血管內科"}, {"0203", "新陳代謝科"}}, got)
}

func TestParseFemhDept(t *testing.T) {
	require.Equal(t, []string{"10001", "10002"}, parseFemhDept(readFemhFixture(t, "dept_0203.html")))
}

func TestParseFemhProfile(t *testing.T) {
	name, education, err := parseFemhProfile(readFemhFixture(t, "profile_10001.html"))
	require.NoError(t, err)
	require.Equal(t, "王小明", name)
	require.Equal(t, []string{"輔仁大學公共衛生學系碩士", "國立台灣大學醫學系學士"}, education)

	name, education, err = parseFemhProfile(readFemhFixture(t, "profile_10003.html"))
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

	got, err := NewFemh(srv.URL, NewFetcher(0)).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}
