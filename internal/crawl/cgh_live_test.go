//go:build live

package crawl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestCghLive checks that the real sites still have the structure the synthetic
// fixtures copy. It sends at most 15 requests (4 robots.txt, the main index, one
// list page, one profile, 3 branch indexes, up to 3 汐止 department pages) and
// logs counts only, never names or education.
func TestCghLive(t *testing.T) {
	ctx := context.Background()
	s := NewCgh("https://www.cgh.org.tw", "https://sijhih.cgh.org.tw", "https://hsinchu.cgh.org.tw", "https://neihu.cgh.org.tw", NewFetcher(time.Second))
	hq := s.campuses[0]

	index, err := s.fetcher.Get(ctx, hq.url(hq.index))
	require.NoError(t, err)
	lists := parseCghIndex(index)
	t.Logf("main lists: %d", len(lists))
	require.GreaterOrEqual(t, len(lists), 35)

	page, err := s.fetcher.Get(ctx, hq.listURL(lists[0].ID, 1))
	require.NoError(t, err)
	ids, lastPage := parseCghListPage(page)
	t.Logf("first list page 1: doctors=%d lastPage=%d", len(ids), lastPage)
	require.GreaterOrEqual(t, len(ids), 1)
	require.GreaterOrEqual(t, lastPage, 1)

	profile, err := s.fetcher.Get(ctx, hq.profileURL(ids[0], lists[0].ID))
	require.NoError(t, err)
	name, education, err := parseCghProfile(profile)
	require.NoError(t, err)
	t.Logf("first profile: name non-empty=%v, education lines=%d", name != "", len(education))
	require.NotEmpty(t, name)
	require.GreaterOrEqual(t, len(education), 1)

	var sijhihDepts []cghDept
	for i, c := range s.campuses[1:] {
		index, err := s.fetcher.Get(ctx, c.url(c.index))
		require.NoError(t, err)
		depts := parseCghBranchIndex(index)
		t.Logf("%s departments: %d", c.host, len(depts))
		require.GreaterOrEqual(t, len(depts), 10)
		if i == 0 {
			sijhihDepts = depts
		}
	}

	sijhih := s.campuses[1]
	found := false
	for i, d := range sijhihDepts[:3] {
		page, err := s.fetcher.Get(ctx, sijhih.url("category.asp?category_id="+d.ID))
		require.NoError(t, err)
		listIDs := parseCghListIDs(page)
		t.Logf("%s department %d: lists=%d", sijhih.host, i+1, len(listIDs))
		if len(listIDs) > 0 {
			found = true
			break
		}
	}
	require.True(t, found, "none of the first 3 汐止 departments links to a doctor list")
}
