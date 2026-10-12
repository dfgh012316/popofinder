package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mmhFixtureServer serves the synthetic fixtures by path and query; a page
// without a fixture is a 404, and failDID makes that profile page a 500.
func mmhFixtureServer(t *testing.T, failDID string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		switch r.URL.Path {
		case "/find_doctor_list.php":
			serveFixture(w, "mmh", "list.html")
		case "/doctor_view.php":
			if q.Get("did") == failDID {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			serveFixture(w, "mmh", "profile_"+q.Get("did")+".html")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestParseMmhList(t *testing.T) {
	list := readFixture(t, "mmh", "list.html")
	got, err := parseMmhList(list)
	require.NoError(t, err)
	require.Equal(t, []mmhEntry{{"2", "1001", "心臟血管內科系"}, {"5", "1002", "胃腸肝膽科"}, {"247", "1003", ""}}, got)

	_, err = parseMmhList(strings.Replace(list, "a=TP", "a=TS", 1))
	require.Error(t, err)

	_, err = parseMmhList(strings.Replace(list, "共 5 名", "共 6 名", 1))
	require.Error(t, err)
}

func TestParseMmhProfile(t *testing.T) {
	name, education, err := parseMmhProfile(readFixture(t, "mmh", "profile_1001.html"))
	require.NoError(t, err)
	require.Equal(t, "王小明", name)
	require.Equal(t, []string{"國立陽明大學醫學系", "國立陽明大學臨床醫學研究所碩士"}, education)

	// The second line comes after the nested </ul>: a block ending there would drop it.
	name, education, err = parseMmhProfile(readFixture(t, "mmh", "profile_1002.html"))
	require.NoError(t, err)
	require.Equal(t, "李小華", name)
	require.Equal(t, []string{"臺北醫學大學醫學系 醫學士", "國立台灣大學公共衛生研究所碩士"}, education)

	name, education, err = parseMmhProfile(readFixture(t, "mmh", "profile_1003.html"))
	require.NoError(t, err)
	require.Equal(t, "張小美", name)
	require.Nil(t, education)

	_, _, err = parseMmhProfile("<html></html>")
	require.Error(t, err)
}

func TestMmhCrawl(t *testing.T) {
	srv := mmhFixtureServer(t, "")

	got, err := NewMmh(srv.URL, NewFetcher(0)).Crawl(context.Background())
	require.NoError(t, err)

	require.Len(t, got, 3)
	require.Equal(t, "mmh:1001", got[0].Key)
	require.Equal(t, "mmh:1002", got[1].Key)
	require.Equal(t, "mmh:1003", got[2].Key)
	// The list spells this name with a private-use character; the profile does not.
	require.Equal(t, "王小明", got[0].Name)
	require.Equal(t, "台北", got[0].City)
	require.Equal(t, "馬偕紀念醫院", got[0].Hospital)
	require.Equal(t, []string{"心臟血管內科系"}, got[0].Departments)
	require.Regexp(t, `/doctor_view\.php\?depid=2&did=1001$`, got[0].URL)
	require.Nil(t, got[2].Departments)
}

func TestMmhCrawlFailsOnMissingProfile(t *testing.T) {
	srv := mmhFixtureServer(t, "1002")
	f := NewFetcher(0)
	f.sleep = func(time.Duration) {}

	got, err := NewMmh(srv.URL, f).Crawl(context.Background())
	require.Error(t, err)
	require.Nil(t, got)
}
