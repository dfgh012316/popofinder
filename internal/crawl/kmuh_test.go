package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// kmuhFixtureServer serves one hospital's fixtures, named <prefix>_intro.html,
// <prefix>_group_<groupCode>.html, <prefix>_list_<virtualCode>.html and
// <prefix>_profile_<doctorID>.html. A request missing the query the real site
// needs is a 404. replace swaps one fixture name for another.
func kmuhFixtureServer(t *testing.T, prefix string, replace map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		var name string
		switch r.URL.Path {
		case "/Web/WebRegistration/DocIntro/Intro":
			name = "intro.html"
		case "/Web/WebRegistration/Ajax/VirtualDept":
			if q.Get("Controller") == "DocIntro" && q.Get("Action") == "ShowDocList" {
				name = "group_" + q.Get("groupCode") + ".html"
			}
		case "/Web/WebRegistration/DocIntro/ShowDocList":
			if strings.HasPrefix(q.Get("JsonLoc"), `["`) {
				name = "list_" + q.Get("virtualCode") + ".html"
			}
		case "/Web/WebRegistration/DocIntro/DocDetail":
			if q.Get("virtualCode") != "" && q.Get("dCode") != "" {
				name = "profile_" + q.Get("doctorID") + ".html"
			}
		}
		if name == "" {
			http.NotFound(w, r)
			return
		}
		name = prefix + "_" + name
		if to, ok := replace[name]; ok {
			name = to
		}
		if name == "redirect" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		serveFixture(w, "kmuh", name)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestKmuh(main, gangshan *httptest.Server) *Kmuh {
	return NewKmuh(main.URL, gangshan.URL, NewFetcher(0))
}

func TestParseKmuhGroups(t *testing.T) {
	require.Equal(t, []string{"HG07", "HG10"}, parseKmuhGroups(readFixture(t, "kmuh", "main_intro.html")))
}

func TestParseKmuhDepts(t *testing.T) {
	require.Equal(t, []kmuhDept{
		{"HD01", "%5B%220130%22%5D", "心臟血管內科"},
		{"HD07", "%5B%220110%22%5D", "胃腸內科"},
	}, parseKmuhDepts(readFixture(t, "kmuh", "main_group_HG07.html")))
}

func TestParseKmuhDoctorList(t *testing.T) {
	ids, query := parseKmuhDoctorList(readFixture(t, "kmuh", "main_list_HD01.html"))
	require.Equal(t, []string{"900001", "900002"}, ids)
	require.Equal(t, "virtualCode=HD01&dCode=0130", query["900001"])

	ids, _ = parseKmuhDoctorList(readFixture(t, "kmuh", "list_empty.html"))
	require.Nil(t, ids)
}

func TestParseKmuhProfile(t *testing.T) {
	name, education, err := parseKmuhProfile(readFixture(t, "kmuh", "main_profile_900001.html"))
	require.NoError(t, err)
	require.Equal(t, "王小明", name)
	require.Equal(t, []string{"高雄醫學大學醫學系", "高雄醫學大學醫學研究所博士"}, education)

	name, education, err = parseKmuhProfile(readFixture(t, "kmuh", "main_profile_900002.html"))
	require.NoError(t, err)
	require.Equal(t, "李小華", name)
	require.Nil(t, education)

	_, _, err = parseKmuhProfile("<html></html>")
	require.Error(t, err)
}

func TestKmuhCrawlFirstHospitalWins(t *testing.T) {
	main := kmuhFixtureServer(t, "main", nil)
	gangshan := kmuhFixtureServer(t, "gangshan", nil)

	// 900001 is also listed at 岡山: he stays under 中和. HD01 is in two groups
	// and is crawled once.
	got, err := newTestKmuh(main, gangshan).Crawl(context.Background())
	require.NoError(t, err)
	keys := make([]string, len(got))
	for i, d := range got {
		keys[i] = d.Key
	}
	require.Equal(t, []string{"kmuh:900001", "kmuh:900002", "kmuh:900003", "kmuh:900004", "kmuh:910001"}, keys)

	require.Equal(t, "高雄醫學大學附設中和紀念醫院", got[0].Hospital)
	require.Equal(t, "高雄", got[0].City)
	require.Equal(t, []string{"心臟血管內科"}, got[0].Departments)
	require.Equal(t, main.URL+"/Web/WebRegistration/DocIntro/DocDetail?lang=tw&doctorID=900001&virtualCode=HD01&dCode=0130", got[0].URL)
	require.Equal(t, []string{"心臟血管內科", "胃腸內科"}, got[1].Departments)
	require.Equal(t, "高醫岡山醫院", got[4].Hospital)
	require.Equal(t, []string{"心臟內科"}, got[4].Departments)
}

func TestKmuhCrawlSkipsRedirectedProfile(t *testing.T) {
	// Some profiles redirect to the home page instead of showing the doctor.
	main := kmuhFixtureServer(t, "main", map[string]string{"main_profile_900003.html": "redirect"})
	gangshan := kmuhFixtureServer(t, "gangshan", nil)

	got, err := newTestKmuh(main, gangshan).Crawl(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 4)
	for _, d := range got {
		require.NotEqual(t, "kmuh:900003", d.Key)
	}
}

func TestKmuhCrawlFailsWhenHospitalHasNoDoctors(t *testing.T) {
	main := kmuhFixtureServer(t, "main", nil)
	gangshan := kmuhFixtureServer(t, "gangshan", map[string]string{"gangshan_list_GD01.html": "list_empty.html"})

	got, err := newTestKmuh(main, gangshan).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}

func TestKmuhCrawlFailsOnMissingPage(t *testing.T) {
	main := kmuhFixtureServer(t, "main", map[string]string{"main_profile_900003.html": "missing.html"})
	gangshan := kmuhFixtureServer(t, "gangshan", nil)

	got, err := newTestKmuh(main, gangshan).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}
