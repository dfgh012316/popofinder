package search

import "strings"

var cities = []string{
	"南投", "台中", "台北", "台南", "台東", "嘉義", "基隆",
	"宜蘭", "屏東", "彰化", "新北", "新竹", "桃園", "花蓮", "苗栗", "雲林", "高雄",
}

// ParseCriteria parses a user message into search criteria.
// Supported formats:
//
//	[城市]@醫院 醫院名稱   e.g. 台北@醫院 台大醫院
//	[城市]@科別 科別名稱   e.g. 台北@科別 小兒科
//	[城市] 關鍵字          e.g. 台北 陳 / 三總 / 牙科（同時比對姓名、醫院、科別）
//
// Words separated by spaces must all match (三總 牙科, docs/open-decisions.md OD-14).
// The city may also have 市/縣 after it, and in a keyword search it may be any word
// (陳大文 桃園), and a word that starts with a hospital alias is split after it
// (三總牙科). An @ not followed by 醫院 or 科別 is dropped (@長庚). A first word that is
// a hospital alias (台中榮總) is not split into a city and a keyword.
func ParseCriteria(message string) Criteria {
	// Data and users spell 台 and 臺 both ways; search with 台 only. Repository.Search
	// reads 臺 as 台 in the columns it compares (docs/open-decisions.md OD-13).
	words := strings.Fields(strings.ReplaceAll(message, "臺", "台"))
	searchType := TypeAll
	var city *string

	// Check for city prefix
	if len(words) > 0 && !isHospitalAlias(words[0]) {
		for _, c := range cities {
			if rest, ok := strings.CutPrefix(words[0], c); ok {
				tmp := c
				city = &tmp
				if rest == "市" || rest == "縣" {
					rest = ""
				}
				words = replaceFirst(words, rest)
				break
			}
		}
	}

	// Check search type
	if len(words) > 0 {
		if rest, ok := strings.CutPrefix(words[0], "@醫院"); ok {
			searchType = TypeHospital
			words = replaceFirst(words, rest)
		} else if rest, ok := strings.CutPrefix(words[0], "@科別"); ok {
			searchType = TypeDepartment
			words = replaceFirst(words, rest)
		} else if rest, ok := strings.CutPrefix(words[0], "@"); ok {
			words = replaceFirst(words, rest)
		}
	}

	if searchType == TypeAll {
		// A city word anywhere in a keyword search
		if city == nil {
			for i, w := range words {
				if c := cityOf(w); c != "" {
					city = &c
					words = append(words[:i:i], words[i+1:]...)
					break
				}
			}
		}
		// A hospital alias run together with more (三總牙科)
		var split []string
		for _, w := range words {
			split = append(split, splitHospitalAlias(w)...)
		}
		words = split
	}

	return Criteria{
		SearchType: searchType,
		SearchTerm: strings.Join(words, " "),
		City:       city,
	}
}

// replaceFirst sets the first word to w, dropping it when w is empty.
func replaceFirst(words []string, w string) []string {
	if w == "" {
		return words[1:]
	}
	words[0] = w
	return words
}

// cityOf returns the city a word names (台北, 台北市), or "".
func cityOf(w string) string {
	for _, c := range cities {
		if w == c || w == c+"市" || w == c+"縣" {
			return c
		}
	}
	return ""
}
