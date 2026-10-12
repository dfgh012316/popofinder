package crawl

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestIsMedicalDegreeLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"國立台灣大學醫學系", true},
		{"緬甸仰光第一醫學院醫學系學士", true},
		{"西班牙埃雷拉大學牙醫學士", true},
		{"中國醫藥大學中醫學系", true},
		{"高雄醫學大學學士後醫學系", true},
		{"長庚大學醫學學士", true},
		{"波蘭盧布林醫學大學", true},
		{"國立陽明大學醫學院", true},
		{"Doctor of Medicine, University of the Philippines", true},
		{"M.D., Universidad Central del Este", true},
		{"輔仁大學公共衛生學系碩士", false},
		{"台北醫學大學醫學研究所博士", false},
		{"高雄醫學大學藥學系", false},
		{"國立陽明大學護理學系", false},
		{"Master of Public Health, Johns Hopkins University", false},
	}
	for _, c := range cases {
		t.Run(c.line, func(t *testing.T) {
			require.Equal(t, c.want, isMedicalDegreeLine(c.line))
		})
	}
}

func TestHasMedicalDegree(t *testing.T) {
	require.True(t, hasMedicalDegree([]string{"輔仁大學公共衛生學系碩士", "國立台灣大學醫學系"}))
	require.False(t, hasMedicalDegree([]string{"輔仁大學公共衛生學系碩士"}))
	require.False(t, hasMedicalDegree(nil))
}

func TestFormatEducation(t *testing.T) {
	require.Equal(t, "A；B", formatEducation([]string{" A ", "  ", "B"}))

	got := formatEducation([]string{strings.Repeat("醫", 600)})
	require.Equal(t, 500, utf8.RuneCountInString(got))
}
