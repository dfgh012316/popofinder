package template

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/dfgh012316/popofinder/internal/database"
)

func strOrDefault(ns sql.NullString) string {
	if ns.Valid && ns.String != "" {
		return ns.String
	}
	return "未提供"
}

func strVal(s string) string {
	if s == "" {
		return "未提供"
	}
	return s
}

func sourceLabel(ns sql.NullString) string {
	if ns.Valid {
		switch ns.String {
		case "blog":
			return "網路公開彙整"
		case "public_report":
			return "民眾回報"
		}
	}
	return "尚未分類"
}

func doctorBubble(p database.MedicalPersonnel) map[string]any {
	row := func(label, value string, extra map[string]any) map[string]any {
		labelNode := map[string]any{
			"type":  "text",
			"text":  label,
			"size":  "sm",
			"color": "#666666",
			"flex":  2,
		}
		valueNode := map[string]any{
			"type":  "text",
			"text":  value,
			"size":  "sm",
			"color": "#4A4A4A",
			"flex":  4,
		}
		for k, v := range extra {
			valueNode[k] = v
		}
		box := map[string]any{
			"type":     "box",
			"layout":   "horizontal",
			"contents": []any{labelNode, valueNode},
			"spacing":  "md",
		}
		return box
	}

	cityRow := row("縣市", strVal(p.City), nil)

	hospitalRow := row("醫院", strVal(p.Hospital), map[string]any{"wrap": true})
	hospitalRow["margin"] = "md"

	deptRow := row("科別", strOrDefault(p.Department), nil)
	deptRow["margin"] = "md"

	eduRow := row("學歷", strOrDefault(p.Education), map[string]any{"wrap": true})
	eduRow["margin"] = "md"

	sourceRow := row("資料來源", sourceLabel(p.Source), nil)
	sourceRow["margin"] = "md"

	bodyContents := []any{cityRow, hospitalRow, deptRow, eduRow, sourceRow}
	if p.SourceURL.Valid && p.SourceURL.String != "" {
		urlRow := row("來源連結", "點此查看來源", map[string]any{
			"wrap":  true,
			"color": "#0066CC",
			"action": map[string]any{
				"type":  "uri",
				"label": "查看來源",
				"uri":   p.SourceURL.String,
			},
		})
		urlRow["margin"] = "md"
		bodyContents = append(bodyContents, urlRow)
	}

	return map[string]any{
		"type": "bubble",
		"header": map[string]any{
			"type":   "box",
			"layout": "vertical",
			"contents": []any{
				map[string]any{
					"type":   "text",
					"text":   p.Name,
					"size":   "xl",
					"weight": "bold",
					"color":  "#4A4A4A",
				},
			},
			"backgroundColor": "#F0F8FF",
		},
		"body": map[string]any{
			"type":            "box",
			"layout":          "vertical",
			"contents":        bodyContents,
			"backgroundColor": "#FFFFFF",
		},
	}
}

// DoctorsFlexMessage builds the raw JSON for a flex message containing a carousel
// of doctor cards, or a "no results" bubble if the list is empty.
func DoctorsFlexMessage(personnel []database.MedicalPersonnel) (json.RawMessage, error) {
	var altText string
	var contents map[string]any

	if len(personnel) == 0 {
		altText = "找不到相關資料"
		contents = map[string]any{
			"type": "bubble",
			"body": map[string]any{
				"type":   "box",
				"layout": "vertical",
				"contents": []any{
					map[string]any{
						"type":   "text",
						"text":   "找不到相關資料",
						"size":   "lg",
						"weight": "bold",
						"align":  "center",
						"color":  "#666666",
					},
				},
			},
		}
	} else {
		altText = fmt.Sprintf("找到 %d 筆相關資料", len(personnel))
		bubbles := make([]any, len(personnel))
		for i, p := range personnel {
			bubbles[i] = doctorBubble(p)
		}
		contents = map[string]any{
			"type":     "carousel",
			"contents": bubbles,
		}
	}

	return buildFlexMessage(altText, contents)
}

// NextPageFlexMessage builds a postback button to load the next page of results.
func NextPageFlexMessage(currentPage, totalPages, nextOffset int) (json.RawMessage, error) {
	contents := map[string]any{
		"type": "bubble",
		"body": map[string]any{
			"type":   "box",
			"layout": "vertical",
			"contents": []any{
				map[string]any{
					"type":  "text",
					"text":  fmt.Sprintf("目前在第 %d/%d 頁", currentPage, totalPages),
					"size":  "sm",
					"wrap":  true,
					"align": "center",
				},
				map[string]any{
					"type": "button",
					"action": map[string]any{
						"type":  "postback",
						"label": "顯示下一頁",
						"data":  fmt.Sprintf("action=next_page&offset=%d", nextOffset),
					},
					"style":  "primary",
					"margin": "md",
				},
			},
		},
	}
	return buildFlexMessage("顯示更多", contents)
}

func buildFlexMessage(altText string, contents map[string]any) (json.RawMessage, error) {
	contentsJSON, err := json.Marshal(contents)
	if err != nil {
		return nil, err
	}
	msg := map[string]any{
		"type":     "flex",
		"altText":  altText,
		"contents": json.RawMessage(contentsJSON),
	}
	return json.Marshal(msg)
}
