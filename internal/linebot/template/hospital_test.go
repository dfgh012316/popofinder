package template

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/search"
	"github.com/stretchr/testify/require"
)

// Tapping a hospital must run a hospital search for exactly that hospital and city.
func TestHospitalListFlexMessageButtonsSearchHospital(t *testing.T) {
	hospitals := []database.DirectoryHospital{
		{City: "台北", Hospital: "甲綜合醫院", Count: 384},
		{City: "新北", Hospital: "汐止甲綜合醫院", Count: 75},
	}
	raw, err := HospitalListFlexMessage(hospitals)
	require.NoError(t, err)

	var msg struct {
		Contents struct {
			Body struct {
				Contents []struct {
					Type   string            `json:"type"`
					Action map[string]string `json:"action"`
				} `json:"contents"`
			} `json:"body"`
		} `json:"contents"`
	}
	require.NoError(t, json.Unmarshal(raw, &msg))

	var labels []string
	for _, c := range msg.Contents.Body.Contents {
		if c.Type != "button" {
			continue
		}
		require.Equal(t, "message", c.Action["type"])
		require.LessOrEqual(t, utf8.RuneCountInString(c.Action["label"]), 20, "label too long: %s", c.Action["label"])
		h := hospitals[len(labels)]
		city := h.City
		require.Equal(t, search.Criteria{SearchType: search.TypeHospital, SearchTerm: h.Hospital, City: &city}, search.ParseCriteria(c.Action["text"]))
		labels = append(labels, c.Action["label"])
	}
	require.Equal(t, []string{"甲綜合醫院 384 位", "汐止甲綜合醫院 75 位"}, labels)
	require.Contains(t, string(raw), "只收官網有寫學歷的醫師")
	require.Contains(t, string(raw), "其他醫院與診所來自網路公開彙整與民眾回報，只有部分醫師")
}
