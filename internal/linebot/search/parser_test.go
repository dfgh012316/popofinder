package search

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func strPtr(s string) *string { return &s }

func TestParseCriteria(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType SearchType
		wantTerm string
		wantCity *string
	}{
		{
			name:     "keyword search no city",
			input:    "陳",
			wantType: TypeAll,
			wantTerm: "陳",
			wantCity: nil,
		},
		{
			name:     "keyword search with city",
			input:    "台北 陳",
			wantType: TypeAll,
			wantTerm: "陳",
			wantCity: strPtr("台北"),
		},
		{
			name:     "hospital search with city",
			input:    "台北@醫院 台大醫院",
			wantType: TypeHospital,
			wantTerm: "台大醫院",
			wantCity: strPtr("台北"),
		},
		{
			name:     "department search with city",
			input:    "高雄@科別 牙科",
			wantType: TypeDepartment,
			wantTerm: "牙科",
			wantCity: strPtr("高雄"),
		},
		{
			name:     "hospital search no city",
			input:    "@醫院 三總",
			wantType: TypeHospital,
			wantTerm: "三總",
			wantCity: nil,
		},
		{
			name:     "department search no city",
			input:    "@科別 小兒科",
			wantType: TypeDepartment,
			wantTerm: "小兒科",
			wantCity: nil,
		},
		{
			name:     "hospital name without prefix",
			input:    "三總",
			wantType: TypeAll,
			wantTerm: "三總",
			wantCity: nil,
		},
		{
			name:     "臺 city and hospital term",
			input:    "臺北@醫院 臺大醫院",
			wantType: TypeHospital,
			wantTerm: "台大醫院",
			wantCity: strPtr("台北"),
		},
		{
			name:     "臺 city with keyword",
			input:    "臺中 陳",
			wantType: TypeAll,
			wantTerm: "陳",
			wantCity: strPtr("台中"),
		},
		// OD-14
		{
			name:     "several words",
			input:    " 高醫   岡山醫院 ",
			wantType: TypeAll,
			wantTerm: "高醫 岡山醫院",
		},
		{
			name:     "city word after the keyword",
			input:    "陳大文 桃園",
			wantType: TypeAll,
			wantTerm: "陳大文",
			wantCity: strPtr("桃園"),
		},
		{
			name:     "city word with 市 in the middle",
			input:    "王小明 台中市 牙科",
			wantType: TypeAll,
			wantTerm: "王小明 牙科",
			wantCity: strPtr("台中"),
		},
		{
			name:     "city prefix with 市",
			input:    "台北市 陳",
			wantType: TypeAll,
			wantTerm: "陳",
			wantCity: strPtr("台北"),
		},
		{
			name:     "only the first city word is the city",
			input:    "台北 台中",
			wantType: TypeAll,
			wantTerm: "台中",
			wantCity: strPtr("台北"),
		},
		{
			name:     "市 that starts a word is kept",
			input:    "台南市立醫院",
			wantType: TypeAll,
			wantTerm: "市立醫院",
			wantCity: strPtr("台南"),
		},
		{
			name:     "hospital alias is not split into city and keyword",
			input:    "臺中榮總 心臟",
			wantType: TypeAll,
			wantTerm: "台中榮總 心臟",
		},
		{
			name:     "bare @ is dropped",
			input:    "@長庚",
			wantType: TypeAll,
			wantTerm: "長庚",
		},
		{
			name:     "bare @ after a city",
			input:    "高雄@骨科",
			wantType: TypeAll,
			wantTerm: "骨科",
			wantCity: strPtr("高雄"),
		},
		{
			name:     "bare @ before a city",
			input:    "@桃園",
			wantType: TypeAll,
			wantTerm: "",
			wantCity: strPtr("桃園"),
		},
		{
			name:     "@醫院 without a space",
			input:    "@醫院三總",
			wantType: TypeHospital,
			wantTerm: "三總",
		},
		{
			name:     "space between city and @醫院",
			input:    "台北 @醫院 三總",
			wantType: TypeHospital,
			wantTerm: "三總",
			wantCity: strPtr("台北"),
		},
		{
			name:     "hospital alias run together with a department",
			input:    "三總牙科",
			wantType: TypeAll,
			wantTerm: "三總 牙科",
		},
		{
			name:     "longest alias, after a city prefix",
			input:    "台北榮總心臟科",
			wantType: TypeAll,
			wantTerm: "榮總 心臟科",
			wantCity: strPtr("台北"),
		},
		{
			name:     "a whole alias is not split",
			input:    "中國醫藥大學附設醫院",
			wantType: TypeAll,
			wantTerm: "中國醫藥大學附設醫院",
		},
		{
			name:     "a city word in a hospital search stays a hospital word",
			input:    "@醫院 桃園",
			wantType: TypeHospital,
			wantTerm: "桃園",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCriteria(tt.input)
			assert.Equal(t, tt.wantType, got.SearchType)
			assert.Equal(t, tt.wantTerm, got.SearchTerm)
			assert.Equal(t, tt.wantCity, got.City)
		})
	}
}
