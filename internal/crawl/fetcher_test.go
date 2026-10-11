package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFetcherSendsUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			gotUA = r.Header.Get("User-Agent")
			w.Write([]byte("hello"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	body, err := NewFetcher(0).Get(context.Background(), srv.URL+"/ok")
	require.NoError(t, err)
	require.Equal(t, "hello", body)
	require.Equal(t, userAgent, gotUA)
}

func TestFetcherRespectsRobotsDisallow(t *testing.T) {
	hitPrivate := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nDisallow: /private\n"))
		case "/private":
			hitPrivate = true
			w.Write([]byte("secret"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	_, err := NewFetcher(0).Get(context.Background(), srv.URL+"/private")
	require.ErrorContains(t, err, "robots.txt")
	require.False(t, hitPrivate)
}

func TestFetcherAllowsWhenRobotsMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.Write([]byte("hello"))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	body, err := NewFetcher(0).Get(context.Background(), srv.URL+"/ok")
	require.NoError(t, err)
	require.Equal(t, "hello", body)
}

func TestFetcherTreatsRobots5xxAsDisallow(t *testing.T) {
	hitOK := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.WriteHeader(http.StatusServiceUnavailable)
		case "/ok":
			hitOK = true
			w.Write([]byte("hello"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := NewFetcher(0)
	f.sleep = func(time.Duration) {}
	_, err := f.Get(context.Background(), srv.URL+"/ok")
	require.ErrorContains(t, err, "robots.txt")
	require.False(t, hitOK)
}

func TestFetcherErrorsOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	f := NewFetcher(0)
	f.sleep = func(time.Duration) {}
	_, err := f.Get(context.Background(), srv.URL+"/fail")
	require.ErrorContains(t, err, "status 500")
}

func TestFetcherWaitsCrawlDelay(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nCrawl-delay: 5\n"))
		case "/ok":
			w.Write([]byte("hello"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	f := NewFetcher(0)
	var waited time.Duration
	f.sleep = func(d time.Duration) { waited = max(waited, d) }

	_, err := f.Get(context.Background(), srv.URL+"/ok")
	require.NoError(t, err)
	_, err = f.Get(context.Background(), srv.URL+"/ok")
	require.NoError(t, err)
	require.Greater(t, waited, 4*time.Second)
}

// Every request to a host, the robots.txt fetch included, must be preceded by a
// wait once the host has been contacted: no two requests may hit it back-to-back.
func TestFetcherSpacesEveryRequestToHost(t *testing.T) {
	var mu sync.Mutex
	var events []string
	record := func(e string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, e)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record("GET " + r.URL.Path)
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	f := NewFetcher(time.Second)
	f.sleep = func(time.Duration) { record("sleep") }
	_, err := f.Get(context.Background(), srv.URL+"/a")
	require.NoError(t, err)
	_, err = f.Get(context.Background(), srv.URL+"/b")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"GET /robots.txt", "sleep", "GET /a", "sleep", "GET /b"}, events)
}

// A redirect hop would bypass the robots.txt check, so it is never followed.
func TestFetcherDoesNotFollowRedirects(t *testing.T) {
	hitPrivate := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nDisallow: /private\n"))
		case "/old":
			http.Redirect(w, r, "/private", http.StatusFound)
		case "/private":
			hitPrivate = true
			w.Write([]byte("secret"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	_, err := NewFetcher(0).Get(context.Background(), srv.URL+"/old")
	require.ErrorContains(t, err, "status 302")
	require.False(t, hitPrivate)
}

// An over-limit body must fail rather than be parsed truncated, on every request
// path: a truncated page drops doctors, a truncated robots.txt drops rules.
func TestFetcherRejectsOversizedBody(t *testing.T) {
	cases := []struct {
		name        string
		robotsBytes int
		pageBytes   int
		wantErr     bool
	}{
		{"page at limit", 0, maxBodyBytes, false},
		{"page over limit", 0, maxBodyBytes + 1, true},
		{"robots.txt over limit", maxBodyBytes + 1, 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hitPage := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/robots.txt":
					if c.robotsBytes == 0 {
						http.NotFound(w, r)
						return
					}
					w.Write([]byte("User-agent: *\n" + strings.Repeat("#", c.robotsBytes-len("User-agent: *\n"))))
				case "/page":
					hitPage = true
					w.Write([]byte(strings.Repeat("a", c.pageBytes)))
				default:
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()

			f := NewFetcher(0)
			var waits []time.Duration
			f.sleep = func(d time.Duration) { waits = append(waits, d) }
			body, err := f.Get(context.Background(), srv.URL+"/page")
			if !c.wantErr {
				require.NoError(t, err)
				require.Len(t, body, c.pageBytes)
				return
			}
			require.ErrorContains(t, err, "exceeds")
			require.Empty(t, waits, "an over-limit body is not retried")
			if c.robotsBytes > 0 {
				require.False(t, hitPage)
			}
		})
	}
}

func TestFetcherRetriesServerErrors(t *testing.T) {
	cases := []struct {
		name      string
		failures  int // 500s before the page succeeds
		status    int
		wantErr   string
		wantHits  int
		wantWaits []time.Duration
	}{
		{"transient 500 then 200", 2, http.StatusInternalServerError, "", 3, retryWaits},
		{"500 on every attempt", 99, http.StatusInternalServerError, "status 500", 3, retryWaits},
		{"404 is not retried", 99, http.StatusNotFound, "status 404", 1, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			hits := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/robots.txt":
					http.NotFound(w, r)
				case "/page":
					hits++
					if hits <= c.failures {
						w.WriteHeader(c.status)
						return
					}
					w.Write([]byte("ok"))
				}
			}))
			defer srv.Close()

			f := NewFetcher(0)
			var waits []time.Duration
			f.sleep = func(d time.Duration) { waits = append(waits, d) }
			body, err := f.Get(context.Background(), srv.URL+"/page")
			if c.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, "ok", body)
			} else {
				require.ErrorContains(t, err, c.wantErr)
			}
			require.Equal(t, c.wantHits, hits)
			require.Equal(t, c.wantWaits, waits)
		})
	}
}
