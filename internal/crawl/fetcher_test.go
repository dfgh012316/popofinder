package crawl

import (
	"context"
	"net/http"
	"net/http/httptest"
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

	_, err := NewFetcher(0).Get(context.Background(), srv.URL+"/ok")
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

	_, err := NewFetcher(0).Get(context.Background(), srv.URL+"/fail")
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
