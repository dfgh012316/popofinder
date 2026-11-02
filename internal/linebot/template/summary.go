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

// MatchGroup is how many results matched in one column, e.g. 姓名 36.
type MatchGroup struct {
	Label string
	Count int
}

// Summary describes a search's results. LINE writes it as one text (Text); the web
// page lays the same parts out (OD-17).
type Summary struct {
	City        string // "" for all of Taiwan
	Field       string // 醫院 or 科別 for an @醫院 or @科別 search, else ""
	Term        string
	Total       int
	Groups      []MatchGroup // keyword searches only, groups with matches
	First, Last int          // the results on this page, from 1; 0 without results
}

// NewSummary summarises one page of a search's results.
func NewSummary(criteria search.Criteria, stats database.SearchStats) Summary {
	s := Summary{
		Field: map[search.SearchType]string{search.TypeHospital: "醫院", search.TypeDepartment: "科別"}[criteria.SearchType],
		Term:  criteria.SearchTerm,
		Total: stats.TotalCount,
	}
	if criteria.City != nil {
		s.City = *criteria.City
	}
	if stats.TotalCount == 0 {
		return s
	}
	if criteria.SearchType == search.TypeAll {
		for _, g := range []MatchGroup{
			{"姓名", stats.MatchCounts.Name},
			{"醫院", stats.MatchCounts.Hospital},
			{"科別", stats.MatchCounts.Department},
		} {
			if g.Count > 0 {
				s.Groups = append(s.Groups, g)
			}
		}
	}
	s.First = (stats.CurrentPage-1)*database.PageSize + 1
	s.Last = min(stats.CurrentPage*database.PageSize, stats.TotalCount)
	return s
}

// Header is the summary's first line, e.g. 查詢在台北醫院「三總」.
func (s Summary) Header() string {
	location := "全台"
	if s.City != "" {
		location = "在" + s.City
	}
	return fmt.Sprintf("查詢%s%s「%s」", location, s.Field, s.Term)
}

// Text is the summary as LINE sends it above the results.
func (s Summary) Text() string {
	if s.Total == 0 {
		return s.Header() + "\n共有 0 筆符合的結果"
	}
	countLine := fmt.Sprintf("共有 %d 筆符合的結果", s.Total)
	if len(s.Groups) > 0 {
		groups := make([]string, len(s.Groups))
		for i, g := range s.Groups {
			groups[i] = fmt.Sprintf("%s %d", g.Label, g.Count)
		}
		countLine += "（" + strings.Join(groups, "、") + "）"
	}
	return fmt.Sprintf("%s\n%s\n目前顯示第 %d - %d 筆\n%s", s.Header(), countLine, s.First, s.Last, CoverageNote)
}

// SearchSummary is the text above the results in LINE.
func SearchSummary(criteria search.Criteria, stats database.SearchStats) string {
	return NewSummary(criteria, stats).Text()
}
