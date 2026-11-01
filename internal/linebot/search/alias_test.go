package search

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHospitalAliasGroupsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, g := range hospitalAliasGroups {
		for _, n := range append(append([]string{}, g.names...), g.also...) {
			require.NotContains(t, n, "臺", "%s: write 台, search reads 臺 as 台", n)
			require.Equal(t, n, strings.Join(strings.Fields(n), ""), "%s: a name with a space is never one search word", n)
		}
		for _, n := range g.names {
			require.False(t, seen[n], "%s is in two groups", n)
			seen[n] = true
		}
	}
}

func TestHospitalPatterns(t *testing.T) {
	tests := []struct {
		term string
		want []string
	}{
		{"三總", []string{"三總", "三軍總院", "三軍總醫院", "內湖三總", "國防醫學院三軍總醫院"}},
		{"內湖三總", []string{"三總", "三軍總院", "三軍總醫院", "內湖三總", "國防醫學院三軍總醫院"}},
		{"高醫", []string{"高醫", "高醫附醫", "高雄醫學大學附設醫院", "中和紀念醫院", "高雄醫學大學附設"}},
		{"林長", []string{"林長", "林口長庚"}},
		// 紀念 or 綜合 dropped: matches 林口長庚, 亞東醫院, 國泰醫院 too.
		{"林口長庚紀念醫院", []string{"林口長庚紀念醫院", "林口長庚"}},
		{"國泰綜合醫院", []string{"國泰綜合醫院", "國泰"}},
		// A crawled full name is not an alias: only that hospital and its short forms.
		{"高雄醫學大學附設中和紀念醫院", []string{"高雄醫學大學附設中和紀念醫院", "高雄醫學大學附設中和"}},
		// 醫院 without 紀念 or 綜合: also the full forms.
		{"亞東醫院", []string{"亞東醫院", "亞東紀念醫院", "亞東綜合醫院"}},
		{"緻美牙醫診所", []string{"緻美牙醫診所", "緻美牙醫"}},
		// Too short a core.
		{"甲醫院", []string{"甲醫院"}},
		{"紀念醫院", []string{"紀念醫院"}},
		{"陳", []string{"陳"}},
		{"", []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			assert.Equal(t, tt.want, HospitalPatterns(tt.term))
		})
	}
}

// Crawled full names stay one word, so HospitalPatterns can drop 紀念/綜合 from them.
func TestSplitHospitalAliasKeepsCrawledNames(t *testing.T) {
	for _, name := range []string{"林口長庚紀念醫院", "高雄長庚紀念醫院", "高雄醫學大學附設中和紀念醫院", "國泰綜合醫院", "新北市立土城醫院", "台北長庚紀念醫院"} {
		assert.Equal(t, []string{name}, splitHospitalAlias(name))
	}
	assert.Equal(t, []string{"三總", "牙科"}, splitHospitalAlias("三總牙科"))
	assert.Equal(t, []string{"林長", "心臟內科"}, splitHospitalAlias("林長心臟內科"))
}

func TestCriteriaTerms(t *testing.T) {
	assert.Equal(t, []string{"三總", "牙科"}, Criteria{SearchTerm: "三總 牙科"}.Terms())
	assert.Equal(t, []string{""}, Criteria{SearchTerm: ""}.Terms())
}
