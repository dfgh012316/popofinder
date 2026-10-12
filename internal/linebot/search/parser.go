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
func ParseCriteria(message string) Criteria {
	// Data and users spell 台 and 臺 both ways; search with 台 only. Repository.Search
	// reads 臺 as 台 in the columns it compares (docs/open-decisions.md OD-13).
	message = strings.ReplaceAll(message, "臺", "台")
	searchType := TypeAll
	searchTerm := strings.TrimSpace(message)
	var city *string

	// Check for city prefix
	for _, c := range cities {
		if strings.HasPrefix(message, c) {
			tmp := c
			city = &tmp
			searchTerm = strings.TrimSpace(message[len(c):])
			break
		}
	}

	// Check search type
	switch {
	case strings.HasPrefix(searchTerm, "@醫院 "):
		searchType = TypeHospital
		searchTerm = strings.TrimSpace(searchTerm[len("@醫院 "):])
	case strings.HasPrefix(searchTerm, "@科別 "):
		searchType = TypeDepartment
		searchTerm = strings.TrimSpace(searchTerm[len("@科別 "):])
	}

	return Criteria{
		SearchType: searchType,
		SearchTerm: searchTerm,
		City:       city,
	}
}
