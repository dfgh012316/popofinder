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
			name:     "name search no city",
			input:    "陳",
			wantType: TypeName,
			wantTerm: "陳",
			wantCity: nil,
		},
		{
			name:     "name search with city",
			input:    "台北 陳",
			wantType: TypeName,
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
