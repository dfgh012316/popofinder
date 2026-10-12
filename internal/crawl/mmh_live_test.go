//go:build live

package crawl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestMmhLive checks that the real site still has the structure the synthetic
// fixtures copy. It sends at most 5 requests (robots.txt, the list, up to 3
// profiles) and logs counts only, never names or education.
func TestMmhLive(t *testing.T) {
	ctx := context.Background()
	s := NewMmh("https://www.mmh.org.tw", NewFetcher(time.Second))

	list, err := s.fetcher.Get(ctx, s.listURL())
	require.NoError(t, err)
	entries, err := parseMmhList(list)
	require.NoError(t, err)
	t.Logf("entries with a profile: %d", len(entries))
	require.GreaterOrEqual(t, len(entries), 400)

	found := false
	for i, e := range entries[:3] {
		profile, err := s.fetcher.Get(ctx, s.profileURL(e.DepID, e.DID))
		require.NoError(t, err)
		name, education, err := parseMmhProfile(profile)
		require.NoError(t, err)
		t.Logf("profile %d: name non-empty=%v, education lines=%d", i+1, name != "", len(education))
		require.NotEmpty(t, name)
		if len(education) >= 1 {
			found = true
			break
		}
	}
	require.True(t, found, "none of the first 3 profiles has an education line")
}
