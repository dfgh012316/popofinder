package template

import (
	"fmt"
	"strings"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/search"
)

// CoverageNote ends every summary that has results (OD-11): the data now covers
// doctors educated in Taiwan as well as abroad, so a match by itself means nothing.
const CoverageNote = "收錄台灣與國外畢業的醫師，學歷請以卡片為準"

// SearchSummary is the text above the results, in LINE and on the web (OD-17).
func SearchSummary(criteria search.Criteria, stats database.SearchStats) string {
	var location string
	if criteria.City != nil {
		location = "在" + *criteria.City
	} else {
		location = "全台"
	}

	typeText := map[search.SearchType]string{
		search.TypeAll:        "",
		search.TypeHospital:   "醫院",
		search.TypeDepartment: "科別",
	}[criteria.SearchType]

	header := fmt.Sprintf("查詢%s%s「%s」", location, typeText, criteria.SearchTerm)
	if stats.TotalCount == 0 {
		return header + "\n共有 0 筆符合的結果"
	}

	countLine := fmt.Sprintf("共有 %d 筆符合的結果", stats.TotalCount)
	if criteria.SearchType == search.TypeAll {
		var groups []string
		for _, g := range []struct {
			label string
			n     int
		}{
			{"姓名", stats.MatchCounts.Name},
			{"醫院", stats.MatchCounts.Hospital},
			{"科別", stats.MatchCounts.Department},
		} {
			if g.n > 0 {
				groups = append(groups, fmt.Sprintf("%s %d", g.label, g.n))
			}
		}
		countLine += "（" + strings.Join(groups, "、") + "）"
	}

	start := (stats.CurrentPage-1)*database.PageSize + 1
	end := stats.CurrentPage * database.PageSize
	if end > stats.TotalCount {
		end = stats.TotalCount
	}
	return fmt.Sprintf("%s\n%s\n目前顯示第 %d - %d 筆\n%s", header, countLine, start, end, CoverageNote)
}
