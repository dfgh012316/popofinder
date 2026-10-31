//go:build live

package crawl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestKmuhLive checks that the real 中和 site still has the structure the
// synthetic fixtures copy. It sends at most 7 requests (robots.txt, 醫師介紹, the
// first group, its first department, up to 3 profiles) and logs counts only,
// never names or education.
func TestKmuhLive(t *testing.T) {
	ctx := context.Background()
	f := NewFetcher(time.Second)
	h := NewKmuh("https://www.kmuh.org.tw", "https://www.kmugh.org.tw", f).hospitals[0]

	intro, err := f.Get(ctx, h.base+"/Web/WebRegistration/DocIntro/Intro?lang=tw")
	require.NoError(t, err)
	groups := parseKmuhGroups(intro)
	t.Logf("groups: %d", len(groups))
	require.GreaterOrEqual(t, len(groups), 5)

	page, err := f.Get(ctx, h.base+"/Web/WebRegistration/Ajax/VirtualDept?lang=tw&groupCode="+groups[0]+"&Controller=DocIntro&Action=ShowDocList")
	require.NoError(t, err)
	depts := parseKmuhDepts(page)
	t.Logf("first group departments: %d", len(depts))
	require.GreaterOrEqual(t, len(depts), 1)

	list, err := f.Get(ctx, h.base+"/Web/WebRegistration/DocIntro/ShowDocList?virtualCode="+depts[0].VirtualCode+"&JsonLoc="+depts[0].JsonLoc+"&lang=tw")
	require.NoError(t, err)
	ids, query := parseKmuhDoctorList(list)
	t.Logf("first department doctors: %d", len(ids))
	require.GreaterOrEqual(t, len(ids), 1)

	withEducation := 0
	for _, id := range ids[:min(3, len(ids))] {
		profile, err := f.Get(ctx, h.base+"/Web/WebRegistration/DocIntro/DocDetail?lang=tw&doctorID="+id+"&"+query[id])
		require.NoError(t, err)
		name, education, err := parseKmuhProfile(profile)
		require.NoError(t, err)
		t.Logf("profile: name non-empty=%v, education lines=%d", name != "", len(education))
		if len(education) > 0 {
			withEducation++
		}
	}
	require.GreaterOrEqual(t, withEducation, 1)
}
