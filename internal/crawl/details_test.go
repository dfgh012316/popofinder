package crawl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Each source reads the 專長 and 經歷 fields of its profile pages (OD-18).
func TestDetailFields(t *testing.T) {
	for _, tc := range []struct {
		source, fixture       string
		details               detailFields
		specialty, experience []string
	}{
		{"femh", "profile_10001.html", femhDetails, []string{"糖尿病"}, []string{"亞東紀念醫院新陳代謝科總醫師", "輔仁大學醫學系兼任講師"}},
		{"shh", "profile_10001.html", shhDetails, []string{"高血壓"}, []string{"雙和醫院心臟內科總醫師"}},
		{"mmh", "profile_1001.html", mmhDetails, []string{"胃鏡檢查、大腸鏡檢查"}, []string{"馬偕紀念醫院總醫師"}},
		{"cgh", "main_profile_20001.html", cghDetails, []string{"胃腸疾病,內視鏡檢查"}, []string{"國泰綜合醫院總醫師"}},
		{"cgmh", "doctor_1001.html", cgmhDetails, nil, []string{"長庚紀念醫院 住院醫師"}},
		{"kmuh", "main_profile_900001.html", kmuhDetails, []string{"心臟超音波", "高血壓"}, []string{"高雄醫學大學附設中和紀念醫院內科住院醫師", "高雄醫學大學附設中和紀念醫院心臟內科主治醫師"}},
		{"smile", "doctor_dr-wang-xiao-ming.html", smileDetails, []string{"口腔外科專科", "人工植牙"}, nil},
		{"vghtc", "profile_A001.html", vghtcDetails, []string{"高血壓、心臟衰竭"}, []string{"臺中榮民總醫院 主治醫師 （2015.08-迄今）", "某某醫學大學附設醫院 住院醫師"}},
	} {
		t.Run(tc.source, func(t *testing.T) {
			specialty, experience := tc.details.read(readFixture(t, tc.source, tc.fixture))
			require.Equal(t, tc.specialty, specialty)
			require.Equal(t, tc.experience, experience)
		})
	}
}

// A page without the fields has neither; a 經歷 title with nothing under it is
// not read from the next section.
func TestDetailFieldsMissing(t *testing.T) {
	specialty, experience := kmuhDetails.read(readFixture(t, "kmuh", "main_profile_900002.html"))
	require.Nil(t, specialty)
	require.Nil(t, experience)
	specialty, experience = femhDetails.read("<html></html>")
	require.Nil(t, specialty)
	require.Nil(t, experience)
}

func TestFormatLines(t *testing.T) {
	require.Equal(t, "甲；乙", formatLines([]string{" 甲 ", "", "乙"}, 10))
	// Whole lines are dropped from the end until it fits.
	require.Equal(t, "一二三；四五", formatLines([]string{"一二三", "四五", "六七八"}, 7))
	// A first line over the limit is cut.
	require.Equal(t, "一二三", formatLines([]string{"一二三四五"}, 3))
	require.Equal(t, "", formatLines(nil, 10))
}
