package template

import (
	"testing"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/search"
	"github.com/stretchr/testify/require"
)

func TestSearchSummary(t *testing.T) {
	taipei := "台北"
	tests := []struct {
		name     string
		criteria search.Criteria
		stats    database.SearchStats
		want     string
	}{
		{
			name:     "all fields, two groups",
			criteria: search.Criteria{SearchType: search.TypeAll, SearchTerm: "林"},
			stats:    database.SearchStats{TotalCount: 5, CurrentPage: 1, TotalPages: 1, MatchCounts: database.MatchCounts{Name: 2, Hospital: 3}},
			want:     "查詢全台「林」\n共有 5 筆符合的結果（姓名 2、醫院 3）\n目前顯示第 1 - 5 筆\n收錄台灣與國外畢業的醫師，學歷請以卡片為準",
		},
		{
			name:     "all fields with city, second page, zero group omitted",
			criteria: search.Criteria{SearchType: search.TypeAll, SearchTerm: "陳", City: &taipei},
			stats:    database.SearchStats{TotalCount: 12, CurrentPage: 2, TotalPages: 2, MatchCounts: database.MatchCounts{Name: 10, Department: 2}},
			want:     "查詢在台北「陳」\n共有 12 筆符合的結果（姓名 10、科別 2）\n目前顯示第 11 - 12 筆\n收錄台灣與國外畢業的醫師，學歷請以卡片為準",
		},
		{
			name:     "hospital type keeps old format",
			criteria: search.Criteria{SearchType: search.TypeHospital, SearchTerm: "三總", City: &taipei},
			stats:    database.SearchStats{TotalCount: 9, CurrentPage: 1, TotalPages: 1},
			want:     "查詢在台北醫院「三總」\n共有 9 筆符合的結果\n目前顯示第 1 - 9 筆\n收錄台灣與國外畢業的醫師，學歷請以卡片為準",
		},
		{
			name:     "zero results hides page range",
			criteria: search.Criteria{SearchType: search.TypeAll, SearchTerm: "abc"},
			stats:    database.SearchStats{TotalCount: 0, CurrentPage: 1},
			want:     "查詢全台「abc」\n共有 0 筆符合的結果",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, SearchSummary(tt.criteria, tt.stats))
		})
	}
}
