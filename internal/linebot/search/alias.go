package search

import (
	"strings"
	"unicode/utf8"
)

// hospitalAliasGroups lists what people type for one hospital (names) and what else
// its names in the data contain (also). A search term equal to one of the names
// matches hospitals whose name contains any of the names or any of also
// (docs/open-decisions.md OD-14). Written with 台 only: ParseCriteria and
// Repository.Search read 臺 as 台. Never put a crawled hospital's full name here
// when the group also matches other hospitals: the 「同院同科醫師」 button searches
// with that name, and 高雄醫學大學附設中和紀念醫院 must not pull in 高醫岡山醫院. Nor a
// name that starts a crawled full name (林口長庚): the word would be split into
// 林口長庚 and 紀念醫院, and no longer find the blog's 林口長庚 rows.
var hospitalAliasGroups = []struct {
	names, also []string
}{
	{names: []string{"三總", "三軍總院", "三軍總醫院", "內湖三總", "國防醫學院三軍總醫院"}},
	{names: []string{"北榮", "台北榮總", "台北榮民總醫院"}},
	{names: []string{"中榮", "台中榮總", "台中榮民總醫院"}},
	{names: []string{"高榮", "高雄榮總", "高雄榮民總醫院"}},
	{names: []string{"榮總", "榮民總醫院"}},
	{names: []string{"台大", "台大醫院", "台大附醫", "台灣大學醫學院附設醫院"}},
	{names: []string{"北醫", "北醫附醫", "北醫附設醫院", "台北醫學大學附設醫院"}},
	{names: []string{"中國附醫", "中國醫", "中國醫大", "中國醫大附醫", "中醫大", "中醫大附醫", "中國醫藥大學附設醫院"}},
	{names: []string{"中山附醫", "中山醫大", "中山醫大附醫", "中山醫學大學附設醫院"}, also: []string{"中山醫學大學"}},
	{names: []string{"彰基", "彰化基督教醫院"}, also: []string{"彰化基督教"}},
	{names: []string{"員基", "員林基督教醫院"}, also: []string{"員林基督教"}},
	{names: []string{"嘉基", "嘉義基督教醫院"}, also: []string{"嘉義基督教"}},
	{names: []string{"屏基", "屏東基督教醫院"}, also: []string{"屏東基督教"}},
	{names: []string{"成大", "成大醫院", "成大附醫", "成功大學醫學院附設醫院"}},
	{names: []string{"高醫", "高醫附醫", "高雄醫學大學附設醫院"}, also: []string{"中和紀念醫院", "高雄醫學大學附設"}},
	{names: []string{"林長"}, also: []string{"林口長庚"}},
	{names: []string{"高長"}, also: []string{"高雄長庚"}},
	{names: []string{"土城長庚", "土城醫院"}},
	{names: []string{"台北慈濟", "新店慈濟"}},
	{names: []string{"花慈", "花蓮慈濟"}},
	{names: []string{"北市聯醫", "市聯醫", "台北聯醫", "台北市立聯合醫院"}, also: []string{"台北聯合醫院"}},
	{names: []string{"部桃", "署桃", "部立桃園醫院", "衛福部桃園醫院", "衛生福利部桃園醫院"}, also: []string{"桃園醫院"}},
	{names: []string{"部北", "署北", "部立台北醫院", "衛福部台北醫院", "衛生福利部台北醫院"}, also: []string{"台北醫院"}},
	{names: []string{"陽明交大附醫", "陽明附醫", "陽大附醫", "陽明交通大學附設醫院"}},
	{names: []string{"802", "802醫院", "國軍高雄總醫院"}},
}

// hospitalAliases maps each name in hospitalAliasGroups to its group's patterns.
var hospitalAliases = func() map[string][]string {
	m := map[string][]string{}
	for _, g := range hospitalAliasGroups {
		patterns := append(append([]string{}, g.names...), g.also...)
		for _, n := range g.names {
			m[n] = patterns
		}
	}
	return m
}()

// isHospitalAlias reports whether term is a name in hospitalAliasGroups.
func isHospitalAlias(term string) bool {
	_, ok := hospitalAliases[term]
	return ok
}

// splitHospitalAlias splits a word that starts with a hospital alias and goes on
// (三總牙科) into the alias and the rest, using the longest alias. It never loses a
// match: a field containing the whole word contains both parts.
func splitHospitalAlias(w string) []string {
	if isHospitalAlias(w) {
		return []string{w}
	}
	longest := ""
	for name := range hospitalAliases {
		if len(name) > len(longest) && strings.HasPrefix(w, name) {
			longest = name
		}
	}
	if longest == "" {
		return []string{w}
	}
	return []string{longest, w[len(longest):]}
}

// HospitalPatterns returns the strings a hospital name may contain to match term:
// term itself and its alias group, or for a term outside the groups, the same name
// with or without 紀念/綜合. The data writes one hospital both ways (亞東醫院 and
// 亞東紀念醫院, 國泰醫院 and 國泰綜合醫院, 林口長庚 and 林口長庚紀念醫院), and also
// 緻美牙醫 next to 緻美牙醫診所. A core shorter than 2 characters is not used.
func HospitalPatterns(term string) []string {
	if patterns, ok := hospitalAliases[term]; ok {
		return patterns
	}
	longEnough := func(s string) bool { return utf8.RuneCountInString(s) >= 2 }
	for _, suffix := range []string{"紀念醫院", "綜合醫院"} {
		if core, ok := strings.CutSuffix(term, suffix); ok {
			if longEnough(core) {
				return []string{term, core}
			}
			return []string{term}
		}
	}
	if core, ok := strings.CutSuffix(term, "醫院"); ok && longEnough(core) {
		return []string{term, core + "紀念醫院", core + "綜合醫院"}
	}
	if core, ok := strings.CutSuffix(term, "牙醫診所"); ok && longEnough(core) {
		return []string{term, core + "牙醫"}
	}
	return []string{term}
}
