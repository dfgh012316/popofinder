package crawl

import (
	"context"
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

// Fetcher GETs pages with an identifiable User-Agent, obeys each host's robots.txt
// (including Crawl-delay) and spaces out requests to the same host.
// It is not safe for concurrent use.
type Fetcher struct {
	client      *http.Client
	minInterval time.Duration
	sleep       func(time.Duration)
	robots      map[string]*robotstxt.RobotsData // keyed by scheme://host
	last        map[string]time.Time             // keyed by scheme://host
}

func NewFetcher(minInterval time.Duration) *Fetcher {
	return &Fetcher{
		client:      &http.Client{Timeout: 30 * time.Second},
		minInterval: minInterval,
		sleep:       time.Sleep,
		robots:      map[string]*robotstxt.RobotsData{},
		last:        map[string]time.Time{},
	}
}

// Get returns the body of rawURL, or an error if robots.txt disallows it or the
// response status is not 200.
func (f *Fetcher) Get(ctx context.Context, rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("crawl: parse %s: %w", rawURL, err)
	}
	host := u.Scheme + "://" + u.Host

	data, ok := f.robots[host]
	if !ok {
		status, body, err := f.do(ctx, host+"/robots.txt")
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

	status, body, err := f.do(ctx, rawURL)
	f.last[host] = time.Now()
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("crawl: GET %s: status %d", rawURL, status)
	}
	return string(body), nil
}

// do sends one GET with the crawler User-Agent and reads at most maxBodyBytes.
func (f *Fetcher) do(ctx context.Context, rawURL string) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, fmt.Errorf("crawl: GET %s: %w", rawURL, err)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := f.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("crawl: GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return 0, nil, fmt.Errorf("crawl: read %s: %w", rawURL, err)
	}
	return resp.StatusCode, body, nil
}
