package template

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDonateText(t *testing.T) {
	raw, err := DonateTextMessage()
	require.NoError(t, err)
	var m struct{ Type, Text string }
	require.NoError(t, json.Unmarshal(raw, &m))
	require.Equal(t, "text", m.Type)
	for _, want := range []string{"贊助換不到任何東西", "每筆最低新台幣 100 元", "7 天內", ContactEmail, DonateURL} {
		require.Contains(t, m.Text, want)
	}
}
