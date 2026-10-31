package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// smileFixtureServer serves all-doctors/<slug> as branch_<slug>.html and
// doctors/<slug> as doctor_<slug>.html; replace swaps one fixture name for another.
func smileFixtureServer(t *testing.T, replace map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var name string
		if slug, ok := strings.CutPrefix(r.URL.Path, "/all-doctors/"); ok {
			name = "branch_" + slug + ".html"
		} else if slug, ok := strings.CutPrefix(r.URL.Path, "/doctors/"); ok {
			name = "doctor_" + slug + ".html"
		}
		if to, ok := replace[name]; ok {
			name = to
		}
		serveFixture(w, "smile", name)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// newTestSmile crawls only 板橋 and 金山, both served by srv.
func newTestSmile(srv *httptest.Server) *Smile {
	s := NewSmile(srv.URL, NewFetcher(0))
	s.branches = s.branches[:2]
	return s
}

func TestParseSmileBranch(t *testing.T) {
	require.Equal(t, []string{"dr-wang-xiao-ming", "dr-li-xiao-hua"}, parseSmileBranch(readFixture(t, "smile", "branch_banqiaosmile.html")))
	require.Nil(t, parseSmileBranch(readFixture(t, "smile", "branch_empty.html")))
}

func TestParseSmileProfile(t *testing.T) {
	// Bio as a <p> after the heading: posts, certifications and the club line are left out.
	id, name, education, err := parseSmileProfile(readFixture(t, "smile", "doctor_dr-wang-xiao-ming.html"))
	require.NoError(t, err)
	require.Equal(t, "101", id)
	require.Equal(t, "王小明", name)
	require.Equal(t, []string{"臺北醫學大學牙醫學士", "國立陽明大學牙醫系碩士"}, education)

	// Bio inside a rich-text <div>.
	id, name, education, err = parseSmileProfile(readFixture(t, "smile", "doctor_dr-li-xiao-hua.html"))
	require.NoError(t, err)
	require.Equal(t, "102", id)
	require.Equal(t, "李小華", name)
	require.Equal(t, []string{"美國密西根大學牙周病暨人工植牙專科進修", "中山醫學大學牙醫學系學士"}, education)

	_, name, education, err = parseSmileProfile(readFixture(t, "smile", "doctor_dr-chen-da-wen.html"))
	require.NoError(t, err)
	require.Equal(t, "陳大文", name)
	require.Nil(t, education)

	_, _, _, err = parseSmileProfile(readFixture(t, "smile", "doctor_no-shortlink.html"))
	require.Error(t, err)
}

func TestSmileCrawlFirstBranchWins(t *testing.T) {
	srv := smileFixtureServer(t, nil)

	// 王小明 is also listed at 金山: he stays under 板橋.
	got, err := newTestSmile(srv).Crawl(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 3)
	require.Equal(t, []string{"smile:101", "smile:102", "smile:103"}, []string{got[0].Key, got[1].Key, got[2].Key})

	require.Equal(t, "板橋蒔美牙醫診所", got[0].Hospital)
	require.Equal(t, "新北", got[0].City)
	require.Equal(t, []string{"牙科"}, got[0].Departments)
	require.Equal(t, srv.URL+"/doctors/dr-wang-xiao-ming", got[0].URL)

	require.Equal(t, "金山蒔美牙醫診所", got[2].Hospital)
	require.Equal(t, "台北", got[2].City)
}

func TestSmileCrawlFailsWhenBranchListsNoDoctors(t *testing.T) {
	srv := smileFixtureServer(t, map[string]string{"branch_jinshansmile.html": "branch_empty.html"})

	got, err := newTestSmile(srv).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}

func TestSmileCrawlFailsOnMissingPage(t *testing.T) {
	srv := smileFixtureServer(t, map[string]string{"doctor_dr-chen-da-wen.html": "missing.html"})

	got, err := newTestSmile(srv).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}
