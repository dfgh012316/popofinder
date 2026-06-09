package reconcile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type feedResp struct {
	Feed struct {
		Entry []struct {
			Title   struct {
				T string `json:"$t"`
			} `json:"title"`
			Content struct {
				T string `json:"$t"`
			} `json:"content"`
			Link []struct {
				Rel  string `json:"rel"`
				Href string `json:"href"`
			} `json:"link"`
		} `json:"entry"`
	} `json:"feed"`
}

// FetchPlaintextPost fetches the blog's Blogger feed and returns the HTML body
// and permalink of the "純文字版" post.
func FetchPlaintextPost(ctx context.Context, baseURL string) (postHTML string, postURL string, err error) {
	url := baseURL + "/feeds/posts/default?alt=json&max-results=150"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", "", fmt.Errorf("reconcile: new request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("reconcile: fetch feed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("reconcile: feed returned status %d", resp.StatusCode)
	}

	var fr feedResp
	if err := json.NewDecoder(resp.Body).Decode(&fr); err != nil {
		return "", "", fmt.Errorf("reconcile: decode feed: %w", err)
	}

	for _, e := range fr.Feed.Entry {
		if !strings.Contains(e.Title.T, "純文字版") {
			continue
		}
		postURL = ""
		for _, l := range e.Link {
			if l.Rel == "alternate" {
				postURL = l.Href
				break
			}
		}
		return e.Content.T, postURL, nil
	}

	return "", "", fmt.Errorf("reconcile: 純文字版 post not found in feed")
}
