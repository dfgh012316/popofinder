package template

import (
	"encoding/json"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestReportTextMessage(t *testing.T) {
	raw, err := ReportTextMessage()
	require.NoError(t, err)

	var got struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, "text", got.Type, "a text message, so LINE links the address")
	require.LessOrEqual(t, utf8.RuneCountInString(got.Text), 5000, "LINE text message limit")

	// Both ways to add education (OD-15), and takedown as in OD-9.
	for _, want := range []string{ContactEmail, "附上網址", "醫師證書字號", "核對完就刪除", "醫師本人提供", "更正或下架", "可證明身分的資料"} {
		require.Contains(t, got.Text, want)
	}
}
