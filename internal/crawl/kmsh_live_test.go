//go:build live

package crawl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestKmshLive checks that the real 小港 site still has the structure the
// synthetic fixtures copy. It sends at most 6 requests (robots.txt, the index,
// the first department, up to 3 profiles) and logs counts only, never names or
// education.
func TestKmshLive(t *testing.T) {
	ctx := context.Background()
	s := NewKmsh("https://www.kmsh.org.tw/Web/wwwKMHK/dept/8300/2024KMSH/pro", NewFetcher(time.Second))

	index, err := s.fetcher.Get(ctx, s.base+"/doc.asp")
	require.NoError(t, err)
	depts := parseKmshDepts(index)
	t.Logf("departments: %d", len(depts))
	require.GreaterOrEqual(t, len(depts), 10)

	page, err := s.fetcher.Get(ctx, s.base+"/doc_index.asp?dept_code="+depts[0].Code)
	require.NoError(t, err)
	pnos, path := parseKmshDoctors(page)
	t.Logf("first department doctors: %d", len(pnos))
	require.GreaterOrEqual(t, len(pnos), 1)

	withEducation := 0
	for _, pno := range pnos[:min(3, len(pnos))] {
		profile, err := s.fetcher.Get(ctx, s.base+"/"+path[pno])
		require.NoError(t, err)
		name, education, err := parseKmshProfile(profile)
		require.NoError(t, err)
		t.Logf("profile: name non-empty=%v, education lines=%d", name != "", len(education))
		if len(education) > 0 {
			withEducation++
		}
	}
	require.GreaterOrEqual(t, withEducation, 1)
}
