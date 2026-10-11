//go:build live

package crawl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestFemhLive checks that the real site still has the structure the synthetic
// fixtures copy. It sends 4 requests (robots.txt, index, one department, one
// profile) and logs counts only, never names or education.
func TestFemhLive(t *testing.T) {
	ctx := context.Background()
	s := NewFemh("https://www.femh.org.tw", NewFetcher(time.Second))

	index, err := s.fetcher.Get(ctx, s.indexURL())
	require.NoError(t, err)
	depts := parseFemhIndex(index)
	t.Logf("departments: %d", len(depts))
	require.GreaterOrEqual(t, len(depts), 40)

	page, err := s.fetcher.Get(ctx, s.deptURL(depts[0].CID))
	require.NoError(t, err)
	ids := parseFemhDept(page)
	t.Logf("doctors in first department: %d", len(ids))
	require.GreaterOrEqual(t, len(ids), 1)

	profile, err := s.fetcher.Get(ctx, s.profileURL(depts[0].CID, ids[0]))
	require.NoError(t, err)
	name, education, err := parseFemhProfile(profile)
	require.NoError(t, err)
	t.Logf("first profile: name non-empty=%v, education lines=%d", name != "", len(education))
	require.NotEmpty(t, name)
	require.GreaterOrEqual(t, len(education), 1)
}
