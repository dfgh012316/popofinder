package crawl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// cgmhFixtureServer serves the synthetic fixtures: DeptList/<code> as
// deptlist_<code>.html, DeptInfo/<a>/<b>/<c> as dept_<a>_<b>_<c>.html and
// DoctorInfo/<id> as doctor_<id>.html; a page without a fixture (robots.txt
// included) is a 404. replace swaps one fixture name for another.
func cgmhFixtureServer(t *testing.T, replace map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rest, ok := strings.CutPrefix(r.URL.Path, "/tw/Services/")
		if !ok {
			http.NotFound(w, r)
			return
		}
		kind, arg, _ := strings.Cut(rest, "/")
		var name string
		switch kind {
		case "DeptList":
			name = "deptlist_" + arg + ".html"
		case "DeptInfo":
			name = "dept_" + strings.ReplaceAll(arg, "/", "_") + ".html"
		case "DoctorInfo":
			name = "doctor_" + arg + ".html"
		default:
			http.NotFound(w, r)
			return
		}
		if to, ok := replace[name]; ok {
			name = to
		}
		serveFixture(w, "cgmh", name)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newTestCgmh crawls only 林口 and 高雄, both served by srv.
func newTestCgmh(srv *httptest.Server, f *Fetcher) *Cgmh {
	s := NewCgmh(srv.URL, f)
	s.campuses = []cgmhCampus{s.campuses[0], s.campuses[6]}
	return s
}

func TestParseCgmhDeptList(t *testing.T) {
	got := parseCgmhDeptList(readFixture(t, "cgmh", "deptlist_3.html"))
	require.Equal(t, []cgmhDept{{"3/31000/31200", "胃腸肝膽科系"}, {"3/31000/31300", "胸腔內科"}, {"3/32000/32200/32220", "心臟外科"}}, got)
}

func TestParseCgmhDeptPage(t *testing.T) {
	require.Equal(t, []string{"1001", "J001"}, parseCgmhDeptPage(readFixture(t, "cgmh", "dept_3_31000_31200.html")))
	require.Nil(t, parseCgmhDeptPage(readFixture(t, "cgmh", "dept_empty.html")))
}

func TestParseCgmhProfile(t *testing.T) {
	name, education, err := parseCgmhProfile(readFixture(t, "cgmh", "doctor_1001.html"))
	require.NoError(t, err)
	require.Equal(t, "王小明", name)
	require.Equal(t, []string{"長庚大學醫學系畢", "長庚大學臨床醫學研究所博士班畢"}, education)

	name, education, err = parseCgmhProfile(readFixture(t, "cgmh", "doctor_1002.html"))
	require.NoError(t, err)
	require.Equal(t, "張小美", name)
	require.Nil(t, education)

	_, _, err = parseCgmhProfile("<html></html>")
	require.Error(t, err)
}

func TestCgmhCrawlFirstCampusWins(t *testing.T) {
	srv := cgmhFixtureServer(t, nil)

	// 1001 is also listed at 高雄: it stays under 林口 with 林口's departments only.
	got, err := newTestCgmh(srv, NewFetcher(0)).Crawl(context.Background())
	require.NoError(t, err)

	require.Len(t, got, 4)
	require.Equal(t, "cgmh:1001", got[0].Key)
	require.Equal(t, "cgmh:J001", got[1].Key)
	require.Equal(t, "cgmh:1002", got[2].Key)
	require.Equal(t, "cgmh:2001", got[3].Key)

	require.Equal(t, "林口長庚紀念醫院", got[0].Hospital)
	require.Equal(t, "桃園", got[0].City)
	require.Equal(t, []string{"胃腸肝膽科系"}, got[0].Departments)
	require.Equal(t, "王小明", got[0].Name)
	require.Equal(t, srv.URL+"/tw/Services/DoctorInfo/1001", got[0].URL)

	require.Equal(t, []string{"胃腸肝膽科系", "胸腔內科"}, got[1].Departments)

	require.Equal(t, "張小美", got[2].Name)
	require.Equal(t, []string{"胸腔內科", "心臟外科"}, got[2].Departments, "an <h5> sub-department is crawled too")
	require.Nil(t, got[2].Education)

	require.Equal(t, "高雄長庚紀念醫院", got[3].Hospital)
	require.Equal(t, "高雄", got[3].City)
	require.Equal(t, []string{"胃腸肝膽科"}, got[3].Departments)
}

func TestCgmhCrawlFailsWhenCampusHasNoDoctors(t *testing.T) {
	srv := cgmhFixtureServer(t, map[string]string{"dept_8_81000_81200.html": "dept_empty.html"})

	got, err := newTestCgmh(srv, NewFetcher(0)).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}

func TestCgmhCrawlFailsOnMissingPage(t *testing.T) {
	srv := cgmhFixtureServer(t, map[string]string{"doctor_2001.html": "missing.html"})
	f := NewFetcher(0)
	f.sleep = func(time.Duration) {}

	got, err := newTestCgmh(srv, f).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}

// cgmhManyDoctorsServer serves one 林口 department listing doctors 1..total; the
// profiles of the last failing ones return 500.
func cgmhManyDoctorsServer(t *testing.T, total, failing int) (*httptest.Server, *int) {
	t.Helper()
	profileHits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/tw/Services/DeptList/3":
			fmt.Fprint(w, `<a href="/tw/Services/DeptInfo/3/31000/31200"><h4>胸腔內科</h4></a>`)
		case r.URL.Path == "/tw/Services/DeptInfo/3/31000/31200":
			for i := 1; i <= total; i++ {
				fmt.Fprintf(w, `<a href="/tw/Services/DoctorInfo/%d">`, i)
			}
		case strings.HasPrefix(r.URL.Path, "/tw/Services/DoctorInfo/"):
			profileHits++
			n, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/tw/Services/DoctorInfo/"))
			require.NoError(t, err)
			if n > total-failing {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			fmt.Fprintf(w, `<h3 class="fz-32">測試%d醫師</h3>`, n)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &profileHits
}

func newManyDoctorsCgmh(srv *httptest.Server) *Cgmh {
	f := NewFetcher(0)
	f.sleep = func(time.Duration) {}
	s := NewCgmh(srv.URL, f)
	s.campuses = s.campuses[:1]
	return s
}

func TestCgmhCrawlSkipsUnavailableProfiles(t *testing.T) {
	// 18 of 120 is exactly 15%: not over the cap.
	srv, _ := cgmhManyDoctorsServer(t, 120, 18)

	got, err := newManyDoctorsCgmh(srv).Crawl(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 102)
	require.Equal(t, "cgmh:102", got[101].Key)
}

func TestCgmhCrawlFailsWhenTooManyProfilesUnavailable(t *testing.T) {
	srv, _ := cgmhManyDoctorsServer(t, 120, 19)

	// The ratio is checked after every profile: 18 of 119 is already over 15%.
	got, err := newManyDoctorsCgmh(srv).Crawl(context.Background())
	require.ErrorContains(t, err, "18 of 119 profiles unavailable")
	require.Nil(t, got)
}

func TestCgmhCrawlStopsEarlyWhenProfilesUnavailable(t *testing.T) {
	srv, profileHits := cgmhManyDoctorsServer(t, 120, 120)

	_, err := newManyDoctorsCgmh(srv).Crawl(context.Background())
	require.ErrorContains(t, err, "100 of 100 profiles unavailable")
	require.Equal(t, 100*(1+len(retryWaits)), *profileHits, "stops after the 100th profile")
}
