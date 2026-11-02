package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// vghtcFixtureServer serves listSection.jsp as sections.html, regDoctor.jsp?section=<c>
// as dept_<c>.html and regDocSchedule.jsp?drno=<id> as profile_<id>.html; replace
// swaps one fixture name for another.
func vghtcFixtureServer(t *testing.T, replace map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var name string
		switch r.URL.Path {
		case "/register/listSection.jsp":
			name = "sections.html"
		case "/register/regDoctor.jsp":
			name = "dept_" + r.URL.Query().Get("section") + ".html"
		case "/register/regDocSchedule.jsp":
			name = "profile_" + r.URL.Query().Get("drno") + ".html"
		}
		if to, ok := replace[name]; ok {
			name = to
		}
		serveFixture(w, "vghtc", name)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestParseVghtcDepts(t *testing.T) {
	// The 特別門診 group goes last; a link without a name and a repeated code are dropped.
	require.Equal(t, []vghtcDept{{"CV", "心臟內科"}, {"GM", "一般內科"}, {"FM", "家庭醫學"}, {"QUIT", "戒菸"}},
		parseVghtcDepts(readFixture(t, "vghtc", "sections.html")))
}

func TestParseVghtcDoctors(t *testing.T) {
	require.Equal(t, []string{"A001", "A002"}, parseVghtcDoctors(readFixture(t, "vghtc", "dept_CV.html")))
	require.Nil(t, parseVghtcDoctors(readFixture(t, "vghtc", "dept_empty.html")))
}

func TestParseVghtcProfile(t *testing.T) {
	name, education, err := parseVghtcProfile(readFixture(t, "vghtc", "profile_A001.html"))
	require.NoError(t, err)
	require.Equal(t, "王小明", name)
	require.Equal(t, []string{"國立陽明大學 臨床醫學研究所博士", "國立陽明大學 醫學系學士"}, education)

	// Without a 學歷 field the 經歷 field is not read as education.
	name, education, err = parseVghtcProfile(readFixture(t, "vghtc", "profile_A002.html"))
	require.NoError(t, err)
	require.Equal(t, "陳大文", name)
	require.Nil(t, education)

	_, _, err = parseVghtcProfile("<html></html>")
	require.Error(t, err)
}

func TestVghtcCrawl(t *testing.T) {
	srv := vghtcFixtureServer(t, nil)

	got, err := NewVghtc(srv.URL+"/register", NewFetcher(0)).Crawl(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"vghtc:A001", "vghtc:A002", "vghtc:A003"}, []string{got[0].Key, got[1].Key, got[2].Key})
	require.Len(t, got, 3)
	require.Equal(t, "臺中榮民總醫院", got[0].Hospital)
	require.Equal(t, "台中", got[0].City)
	require.Equal(t, srv.URL+"/register/regDocSchedule.jsp?init=init&section=CV&drno=A001", got[0].URL)
	// The special clinic comes after the doctor's own departments.
	require.Equal(t, []string{"心臟內科", "家庭醫學", "戒菸"}, got[0].Departments)
	require.Equal(t, []string{"心臟內科", "一般內科"}, got[1].Departments)
	// A doctor listed only at a special clinic is still crawled.
	require.Equal(t, []string{"戒菸"}, got[2].Departments)
	require.Equal(t, srv.URL+"/register/regDocSchedule.jsp?init=init&section=QUIT&drno=A003", got[2].URL)
}

func TestVghtcCrawlFailsWithoutDoctors(t *testing.T) {
	empty := map[string]string{}
	for _, c := range []string{"CV", "GM", "FM", "QUIT"} {
		empty["dept_"+c+".html"] = "dept_empty.html"
	}
	srv := vghtcFixtureServer(t, empty)

	got, err := NewVghtc(srv.URL+"/register", NewFetcher(0)).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}

func TestVghtcCrawlFailsOnMissingPage(t *testing.T) {
	srv := vghtcFixtureServer(t, map[string]string{"profile_A003.html": "missing.html"})

	got, err := NewVghtc(srv.URL+"/register", NewFetcher(0)).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}
