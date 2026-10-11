package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/search"
	"github.com/dfgh012316/popofinder/internal/linebot/template"
	"github.com/stretchr/testify/require"
)

func TestFormatSummary(t *testing.T) {
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
			want:     "查詢全台「林」\n共有 5 筆符合的結果（姓名 2、醫院 3）\n目前顯示第 1 - 5 筆",
		},
		{
			name:     "all fields with city, second page, zero group omitted",
			criteria: search.Criteria{SearchType: search.TypeAll, SearchTerm: "陳", City: &taipei},
			stats:    database.SearchStats{TotalCount: 12, CurrentPage: 2, TotalPages: 2, MatchCounts: database.MatchCounts{Name: 10, Department: 2}},
			want:     "查詢在台北「陳」\n共有 12 筆符合的結果（姓名 10、科別 2）\n目前顯示第 11 - 12 筆",
		},
		{
			name:     "hospital type keeps old format",
			criteria: search.Criteria{SearchType: search.TypeHospital, SearchTerm: "三總", City: &taipei},
			stats:    database.SearchStats{TotalCount: 9, CurrentPage: 1, TotalPages: 1},
			want:     "查詢在台北醫院「三總」\n共有 9 筆符合的結果\n目前顯示第 1 - 9 筆",
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
			require.Equal(t, tt.want, formatSummary(tt.criteria, tt.stats))
		})
	}
}

func TestZeroResultActions(t *testing.T) {
	taipei := "台北"
	help := template.QuickReplyAction{Label: "使用說明", Text: "幫助"}
	widen := func(term string) template.QuickReplyAction {
		return template.QuickReplyAction{Label: "擴大搜尋", Text: term}
	}
	long300 := strings.Repeat("字", 300)
	tests := []struct {
		name     string
		criteria search.Criteria
		want     []template.QuickReplyAction
	}{
		{"nationwide all fields", search.Criteria{SearchType: search.TypeAll, SearchTerm: "abc"}, []template.QuickReplyAction{help}},
		{"narrowed by city", search.Criteria{SearchType: search.TypeAll, SearchTerm: "abc", City: &taipei}, []template.QuickReplyAction{widen("abc"), help}},
		{"narrowed by @ type", search.Criteria{SearchType: search.TypeHospital, SearchTerm: "三總"}, []template.QuickReplyAction{widen("三總"), help}},
		{"empty term", search.Criteria{SearchType: search.TypeAll, SearchTerm: "", City: &taipei}, []template.QuickReplyAction{help}},
		{"term over 300 runes", search.Criteria{SearchType: search.TypeAll, SearchTerm: strings.Repeat("字", 301), City: &taipei}, []template.QuickReplyAction{help}},
		{"term exactly 300 runes", search.Criteria{SearchType: search.TypeAll, SearchTerm: long300, City: &taipei}, []template.QuickReplyAction{widen(long300), help}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, zeroResultActions(tt.criteria))
		})
	}
}

func TestBuildSearchResponseQuickReply(t *testing.T) {
	taipei := "台北"

	quickReplyTexts := func(t *testing.T, raw json.RawMessage) ([]string, bool) {
		t.Helper()
		var m map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &m))
		qr, ok := m["quickReply"]
		if !ok {
			return nil, false
		}
		var parsed struct {
			Items []struct {
				Action struct {
					Text string `json:"text"`
				} `json:"action"`
			} `json:"items"`
		}
		require.NoError(t, json.Unmarshal(qr, &parsed))
		var texts []string
		for _, item := range parsed.Items {
			texts = append(texts, item.Action.Text)
		}
		return texts, true
	}

	t.Run("zero results", func(t *testing.T) {
		msgs, err := buildSearchResponse(
			search.Criteria{SearchType: search.TypeAll, SearchTerm: "abc", City: &taipei},
			nil,
			database.SearchStats{TotalCount: 0, CurrentPage: 1},
		)
		require.NoError(t, err)
		require.Len(t, msgs, 2)
		_, ok := quickReplyTexts(t, msgs[0])
		require.False(t, ok, "summary message should not carry a quick reply")
		texts, ok := quickReplyTexts(t, msgs[1])
		require.True(t, ok)
		require.Equal(t, []string{"abc", "幫助"}, texts)
	})

	t.Run("with results", func(t *testing.T) {
		msgs, err := buildSearchResponse(
			search.Criteria{SearchType: search.TypeAll, SearchTerm: "林"},
			[]database.MedicalPersonnel{{Name: "測試", City: "台北", Hospital: "測試醫院"}},
			database.SearchStats{TotalCount: 1, CurrentPage: 1, TotalPages: 1, MatchCounts: database.MatchCounts{Name: 1}},
		)
		require.NoError(t, err)
		require.Len(t, msgs, 2)
		for _, m := range msgs {
			_, ok := quickReplyTexts(t, m)
			require.False(t, ok)
		}
	})
}
