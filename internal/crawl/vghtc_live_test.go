//go:build live

package crawl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestVghtcLive checks that the real 中榮 registration system still has the
// structure the synthetic fixtures copy. It sends at most 6 requests (robots.txt,
// the department list, the first department, up to 3 profiles) and logs counts
// only, never names or education.
func TestVghtcLive(t *testing.T) {
	ctx := context.Background()
	s := NewVghtc("https://register.vghtc.gov.tw/register", NewFetcher(time.Second))

	list, err := s.fetcher.Get(ctx, s.base+"/listSection.jsp")
	require.NoError(t, err)
	depts := parseVghtcDepts(list)
	t.Logf("departments: %d", len(depts))
	require.GreaterOrEqual(t, len(depts), 50)
	require.Equal(t, "胸腔內科", depts[0].Name, "first department of the 內科系 group")

	page, err := s.fetcher.Get(ctx, s.base+"/regDoctor.jsp?init=sub&section="+depts[0].Code)
	require.NoError(t, err)
	ids := parseVghtcDoctors(page)
	t.Logf("first department doctors: %d", len(ids))
	require.GreaterOrEqual(t, len(ids), 1)

	withEducation := 0
	for _, id := range ids[:min(3, len(ids))] {
		profile, err := s.fetcher.Get(ctx, s.base+"/regDocSchedule.jsp?init=init&section="+depts[0].Code+"&drno="+id)
		require.NoError(t, err)
		name, education, err := parseVghtcProfile(profile)
		require.NoError(t, err)
		t.Logf("profile: name non-empty=%v, education lines=%d", name != "", len(education))
		if len(education) > 0 {
			withEducation++
		}
	}
	require.GreaterOrEqual(t, withEducation, 1)
}
