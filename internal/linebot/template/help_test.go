package template

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestHelpFlexMessageQuickReply(t *testing.T) {
	raw, err := HelpFlexMessage()
	require.NoError(t, err)

	var got struct {
		QuickReply struct {
			Items []struct {
				Action map[string]string `json:"action"`
			} `json:"items"`
		} `json:"quickReply"`
	}
	require.NoError(t, json.Unmarshal(raw, &got))

	var texts []string
	for _, item := range got.QuickReply.Items {
		require.Equal(t, "message", item.Action["type"])
		require.LessOrEqual(t, utf8.RuneCountInString(item.Action["label"]), 20, "label too long: %s", item.Action["label"])
		texts = append(texts, item.Action["text"])
	}
	require.Equal(t, []string{"陳", "三總", "台北 牙科", "醫院清單"}, texts)
}

func TestHelpFlexMessageUsageText(t *testing.T) {
	raw, err := HelpFlexMessage()
	require.NoError(t, err)
	s := string(raw)

	for _, want := range []string{
		"關鍵字（姓名、醫院或科別），多個條件用空白分開",
		"例如：陳、三總、牙科、高醫 骨科",
		"城市 關鍵字",
		"例如：台北 牙科",
		"[城市]@醫院 醫院名稱",
		"[城市]@科別 科別名稱",
		"輸入「醫院清單」看已收錄官網醫師名單的醫院",
		"例如：台北@科別 心臟（打部分字即可",
		"支援城市",
	} {
		require.Contains(t, s, want)
	}
	require.NotContains(t, s, "[城市] 醫師名稱")
}

func TestHelpFlexMessageTakedownContact(t *testing.T) {
	raw, err := HelpFlexMessage()
	require.NoError(t, err)
	s := string(raw)

	for _, want := range []string{"下架與更正", "jerrychuang@jerrytech.me", "可證明身分的資料"} {
		require.Contains(t, s, want)
	}
}

func TestHelpFlexMessageCoverage(t *testing.T) {
	raw, err := HelpFlexMessage()
	require.NoError(t, err)
	s := string(raw)

	for _, want := range []string{"收錄範圍", "台灣與國外畢業的都有", "查不到也不代表什麼"} {
		require.Contains(t, s, want)
	}
	require.Less(t, strings.Index(s, "收錄範圍"), strings.Index(s, "支援格式"), "coverage comes before the usage formats")
}
