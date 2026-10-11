package template

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithQuickReply(t *testing.T) {
	msg, err := buildFlexMessage("alt", map[string]any{"type": "bubble"})
	require.NoError(t, err)

	raw, err := WithQuickReply(msg, []QuickReplyAction{{Label: "L", Text: "T"}})
	require.NoError(t, err)

	var got struct {
		Type     string `json:"type"`
		AltText  string `json:"altText"`
		Contents struct {
			Type string `json:"type"`
		} `json:"contents"`
		QuickReply struct {
			Items []struct {
				Type   string            `json:"type"`
				Action map[string]string `json:"action"`
			} `json:"items"`
		} `json:"quickReply"`
	}
	require.NoError(t, json.Unmarshal(raw, &got))
	require.Equal(t, "flex", got.Type)
	require.Equal(t, "alt", got.AltText)
	require.Equal(t, "bubble", got.Contents.Type)
	require.Len(t, got.QuickReply.Items, 1)
	require.Equal(t, "action", got.QuickReply.Items[0].Type)
	require.Equal(t, map[string]string{"type": "message", "label": "L", "text": "T"}, got.QuickReply.Items[0].Action)
}
