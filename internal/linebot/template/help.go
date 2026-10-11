package template

import "encoding/json"

var helpContents = map[string]any{
	"type": "bubble",
	"body": map[string]any{
		"type":   "box",
		"layout": "vertical",
		"contents": []any{
			map[string]any{
				"type":   "text",
				"text":   "使用說明",
				"weight": "bold",
				"size":   "xl",
				"color":  "#1DB446",
			},
			map[string]any{
				"type":   "separator",
				"margin": "xxl",
			},
			map[string]any{
				"type":    "box",
				"layout":  "vertical",
				"margin":  "lg",
				"spacing": "sm",
				"contents": []any{
					map[string]any{
						"type":   "text",
						"text":   "支援格式",
						"weight": "bold",
						"size":   "lg",
					},
					// Keyword search format
					map[string]any{
						"type":    "box",
						"layout":  "vertical",
						"spacing": "sm",
						"contents": []any{
							map[string]any{
								"type":   "box",
								"layout": "baseline",
								"contents": []any{
									map[string]any{"type": "text", "text": "🔍", "size": "sm", "color": "#666666", "flex": 1},
									map[string]any{"type": "text", "text": "關鍵字（姓名、醫院或科別）", "size": "sm", "color": "#666666", "flex": 5, "wrap": true},
								},
							},
							map[string]any{
								"type":   "box",
								"layout": "baseline",
								"contents": []any{
									map[string]any{"type": "text", "text": "💡", "size": "sm", "color": "#666666", "flex": 1},
									map[string]any{"type": "text", "text": "例如：陳、三總、牙科", "size": "sm", "color": "#666666", "flex": 5, "wrap": true},
								},
							},
						},
					},
					map[string]any{"type": "separator", "margin": "lg"},
					// City + keyword format
					map[string]any{
						"type":    "box",
						"layout":  "vertical",
						"spacing": "sm",
						"contents": []any{
							map[string]any{
								"type":   "box",
								"layout": "baseline",
								"contents": []any{
									map[string]any{"type": "text", "text": "📍", "size": "sm", "color": "#666666", "flex": 1},
									map[string]any{"type": "text", "text": "城市 關鍵字", "size": "sm", "color": "#666666", "flex": 5, "wrap": true},
								},
							},
							map[string]any{
								"type":   "box",
								"layout": "baseline",
								"contents": []any{
									map[string]any{"type": "text", "text": "💡", "size": "sm", "color": "#666666", "flex": 1},
									map[string]any{"type": "text", "text": "例如：台北 牙科", "size": "sm", "color": "#666666", "flex": 5, "wrap": true},
								},
							},
						},
					},
					map[string]any{"type": "separator", "margin": "lg"},
					// Hospital search format
					map[string]any{
						"type":    "box",
						"layout":  "vertical",
						"spacing": "sm",
						"contents": []any{
							map[string]any{
								"type":   "box",
								"layout": "baseline",
								"contents": []any{
									map[string]any{"type": "text", "text": "🏥", "size": "sm", "color": "#666666", "flex": 1},
									map[string]any{"type": "text", "text": "[城市]@醫院 醫院名稱", "size": "sm", "color": "#666666", "flex": 5},
								},
							},
							map[string]any{
								"type":   "box",
								"layout": "baseline",
								"contents": []any{
									map[string]any{"type": "text", "text": "💡", "size": "sm", "color": "#666666", "flex": 1},
									map[string]any{"type": "text", "text": "例如：台北@醫院 三總", "size": "sm", "color": "#666666", "flex": 5},
								},
							},
						},
					},
					map[string]any{"type": "separator", "margin": "lg"},
					// Department search format
					map[string]any{
						"type":    "box",
						"layout":  "vertical",
						"spacing": "sm",
						"contents": []any{
							map[string]any{
								"type":   "box",
								"layout": "baseline",
								"contents": []any{
									map[string]any{"type": "text", "text": "🏥", "size": "sm", "color": "#666666", "flex": 1},
									map[string]any{"type": "text", "text": "[城市]@科別 科別名稱", "size": "sm", "color": "#666666", "flex": 5},
								},
							},
							map[string]any{
								"type":   "box",
								"layout": "baseline",
								"contents": []any{
									map[string]any{"type": "text", "text": "💡", "size": "sm", "color": "#666666", "flex": 1},
									map[string]any{"type": "text", "text": "例如：台北@科別 牙科", "size": "sm", "color": "#666666", "flex": 5},
								},
							},
						},
					},
					map[string]any{"type": "separator", "margin": "lg"},
					// Supported cities
					map[string]any{
						"type":   "box",
						"layout": "vertical",
						"margin": "lg",
						"contents": []any{
							map[string]any{"type": "text", "text": "支援城市", "weight": "bold", "size": "lg"},
							map[string]any{
								"type":   "box",
								"layout": "vertical",
								"margin": "sm",
								"contents": []any{
									map[string]any{"type": "box", "layout": "horizontal", "contents": []any{map[string]any{"type": "text", "text": "台北、新北、基隆", "size": "sm", "color": "#666666", "flex": 1}}},
									map[string]any{"type": "box", "layout": "horizontal", "contents": []any{map[string]any{"type": "text", "text": "桃園、新竹、苗栗", "size": "sm", "color": "#666666", "flex": 1}}},
									map[string]any{"type": "box", "layout": "horizontal", "contents": []any{map[string]any{"type": "text", "text": "台中、彰化、南投", "size": "sm", "color": "#666666", "flex": 1}}},
									map[string]any{"type": "box", "layout": "horizontal", "contents": []any{map[string]any{"type": "text", "text": "雲林、嘉義、台南", "size": "sm", "color": "#666666", "flex": 1}}},
									map[string]any{"type": "box", "layout": "horizontal", "contents": []any{map[string]any{"type": "text", "text": "高雄、屏東、台東", "size": "sm", "color": "#666666", "flex": 1}}},
									map[string]any{"type": "box", "layout": "horizontal", "contents": []any{map[string]any{"type": "text", "text": "花蓮、宜蘭", "size": "sm", "color": "#666666", "flex": 1}}},
								},
							},
						},
					},
				},
			},
		},
	},
}

// helpQuickReply offers tappable example searches. Examples use a surname, a hospital
// short name, and a department only — never a real doctor's full name (OD-6).
var helpQuickReply = []QuickReplyAction{
	{Label: "範例：陳", Text: "陳"},
	{Label: "範例：三總", Text: "三總"},
	{Label: "範例：台北 牙科", Text: "台北 牙科"},
}

// HelpFlexMessage returns the raw JSON for the help usage message with example quick replies.
func HelpFlexMessage() (json.RawMessage, error) {
	msg, err := buildFlexMessage("使用說明", helpContents)
	if err != nil {
		return nil, err
	}
	return WithQuickReply(msg, helpQuickReply)
}
