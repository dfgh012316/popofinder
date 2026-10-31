//go:build live

package crawl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestCgmhLive checks that the real site still has the structure the synthetic
// fixtures copy. It sends at most 6 requests (robots.txt, 林口's DeptList, its
// first department page, up to 3 profiles: some profiles have no 學歷 list) and
// logs counts only, never names or education.
func TestCgmhLive(t *testing.T) {
	ctx := context.Background()
	s := NewCgmh("https://www.cgmh.org.tw", NewFetcher(time.Second))

	list, err := s.fetcher.Get(ctx, s.base+"/tw/Services/DeptList/"+s.campuses[0].code)
	require.NoError(t, err)
	depts := parseCgmhDeptList(list)
	t.Logf("林口 departments: %d", len(depts))
	require.GreaterOrEqual(t, len(depts), 80)

	page, err := s.fetcher.Get(ctx, s.base+"/tw/Services/DeptInfo/"+depts[0].Path)
	require.NoError(t, err)
	ids := parseCgmhDeptPage(page)
	t.Logf("first department: doctors=%d", len(ids))
	require.GreaterOrEqual(t, len(ids), 1)

	withEducation := 0
	for _, id := range ids[:min(3, len(ids))] {
		profile, err := s.fetcher.Get(ctx, s.profileURL(id))
		require.NoError(t, err)
		name, education, err := parseCgmhProfile(profile)
		require.NoError(t, err)
		t.Logf("profile: name non-empty=%v, education lines=%d", name != "", len(education))
		require.NotEmpty(t, name)
		if len(education) > 0 {
			withEducation++
		}
	}
	require.GreaterOrEqual(t, withEducation, 1)
}
