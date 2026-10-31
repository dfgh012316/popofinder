package crawl

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/temoto/robotstxt"
)

const (
	userAgent    = "popofinder-crawler/1.0 (+mailto:jerrychuang@jerrytech.me)"
	robotsAgent  = "popofinder-crawler"
	maxBodyBytes = 5 << 20
)

// retryWaits are the pauses before retrying a request that hit a network error or
// a 5xx. Hospital sites return occasional 500s, and one failed page fails the whole
// crawl, so a transient error must not end a run of a thousand requests.
var retryWaits = []time.Duration{10 * time.Second, 30 * time.Second}

// errBodyTooLarge is not retried: the same page would be too large again.
var errBodyTooLarge = fmt.Errorf("body exceeds %d bytes", maxBodyBytes)

// errUnavailable marks a page that failed every retry with a network error or a
// 5xx. A source fails on it unless it skips such pages (Cgmh).
var errUnavailable = errors.New("page unavailable after retries")

// Fetcher GETs pages with an identifiable User-Agent, obeys each host's robots.txt
// (including Crawl-delay) and spaces out every request to the same host, the
// robots.txt fetch included. It never follows redirects. It is not safe for
// concurrent use.
type Fetcher struct {
	client      *http.Client
	minInterval time.Duration
	sleep       func(time.Duration)
	robots      map[string]*robotstxt.RobotsData // keyed by scheme://host
	last        map[string]time.Time             // keyed by scheme://host
}

func NewFetcher(minInterval time.Duration) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: 30 * time.Second,
			// A redirect hop would skip the robots.txt check and the spacing in Get,
			// so the 3xx response is returned as-is and fails as a non-200 status.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		minInterval: minInterval,
		sleep:       time.Sleep,
		robots:      map[string]*robotstxt.RobotsData{},
		last:        map[string]time.Time{},
	}
}

// Get returns the body of rawURL, or an error if robots.txt disallows it or the
// response status is not 200. A page that still hits a network error or a 5xx
// after the retries is an errUnavailable error.
func (f *Fetcher) Get(ctx context.Context, rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("crawl: parse %s: %w", rawURL, err)
	}
	host := u.Scheme + "://" + u.Host

	data, ok := f.robots[host]
	if !ok {
		status, body, err := f.doWithRetry(ctx, host, host+"/robots.txt")
		if err != nil {
			return "", err
		}
		data, err = robotstxt.FromStatusAndBytes(status, body)
		if err != nil {
			return "", fmt.Errorf("crawl: parse %s/robots.txt: %w", host, err)
		}
		f.robots[host] = data
	}

	// TestAgent, not FindGroup(...).Test: only TestAgent treats a 5xx robots.txt as disallow-all.
	if !data.TestAgent(u.RequestURI(), robotsAgent) {
		return "", fmt.Errorf("crawl: %s disallowed by robots.txt", rawURL)
	}

	delay := max(f.minInterval, data.FindGroup(robotsAgent).CrawlDelay)
	if last, ok := f.last[host]; ok {
		if w := delay - time.Since(last); w > 0 {
			f.sleep(w)
		}
	}

	status, body, err := f.doWithRetry(ctx, host, rawURL)
	if err != nil {
		if errors.Is(err, errBodyTooLarge) || ctx.Err() != nil {
			return "", err
		}
		return "", fmt.Errorf("%w: %w", err, errUnavailable)
	}
	if status >= 500 {
		return "", fmt.Errorf("crawl: GET %s: status %d: %w", rawURL, status, errUnavailable)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("crawl: GET %s: status %d", rawURL, status)
	}
	return string(body), nil
}

// doWithRetry calls do, retrying after each of retryWaits while the request
// fails with a network error or a 5xx status.
func (f *Fetcher) doWithRetry(ctx context.Context, host, rawURL string) (int, []byte, error) {
	status, body, err := f.do(ctx, host, rawURL)
	for _, w := range retryWaits {
		if errors.Is(err, errBodyTooLarge) || (err == nil && status < 500) {
			break
		}
		f.sleep(w)
		status, body, err = f.do(ctx, host, rawURL)
	}
	return status, body, err
}

// do sends one GET with the crawler User-Agent, records it as the host's last
// request (the spacing in Get counts from there) and returns the body. A body over
// maxBodyBytes is an error: a truncated page would look like a complete one with
// fewer doctors, and a truncated robots.txt could drop Disallow rules.
func (f *Fetcher) do(ctx context.Context, host, rawURL string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("crawl: GET %s: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := f.client.Do(req)
	f.last[host] = time.Now()
	if err != nil {
		return 0, nil, fmt.Errorf("crawl: GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return 0, nil, fmt.Errorf("crawl: read %s: %w", rawURL, err)
	}
	if len(body) > maxBodyBytes {
		return 0, nil, fmt.Errorf("crawl: read %s: %w", rawURL, errBodyTooLarge)
	}
	return resp.StatusCode, body, nil
}
