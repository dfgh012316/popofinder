//go:build live

package crawl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSmileLive checks that the real site still has the structure the synthetic
// fixtures copy. It sends at most 5 requests (robots.txt, 板橋's doctor list, up
// to 3 profiles) and logs counts only, never names or education.
func TestSmileLive(t *testing.T) {
	ctx := context.Background()
	s := NewSmile("https://www.smile-dental.tw", NewFetcher(time.Second))

	list, err := s.fetcher.Get(ctx, s.base+"/all-doctors/"+s.branches[0].slug)
	require.NoError(t, err)
	slugs := parseSmileBranch(list)
	t.Logf("板橋 doctors: %d", len(slugs))
	require.GreaterOrEqual(t, len(slugs), 10)

	withEducation := 0
	for _, slug := range slugs[:3] {
		page, err := s.fetcher.Get(ctx, s.base+"/doctors/"+slug)
		require.NoError(t, err)
		id, name, education, err := parseSmileProfile(page)
		require.NoError(t, err)
		t.Logf("profile: id non-empty=%v, name non-empty=%v, education lines=%d", id != "", name != "", len(education))
		if len(education) > 0 {
			withEducation++
		}
	}
	require.GreaterOrEqual(t, withEducation, 1)
}
