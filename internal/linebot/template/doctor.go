package template

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/dfgh012316/popofinder/internal/database"
)

// SameDepartmentLabel is the button on every doctor card that searches the doctor's
// hospital and department (docs/open-decisions.md OD-14), whatever the education says.
const SameDepartmentLabel = "同院同科醫師"

// SourceLinkLabel labels the row that links to where the data came from.
const SourceLinkLabel = "來源連結"

// NoResultsText is shown when a search finds nothing.
const NoResultsText = "找不到相關資料"

// CardRow is one label and value on a doctor card.
type CardRow struct {
	Label, Value string
	Wrap         bool // the LINE card wraps long values instead of cutting them off
}

// Card is what a doctor card shows. The LINE bubble and the web page (OD-17) both
// render it, so the two show the same fields with the same labels.
type Card struct {
	Name               string
	Rows               []CardRow
	SourceURL          string // "" when there is no source link
	SourceURLText      string
	SameDepartmentText string // what 「同院同科醫師」 searches; "" when there is no button
}

// NewCard builds the card for one doctor.
func NewCard(p database.MedicalPersonnel) Card {
	c := Card{
		Name: p.Name,
		Rows: []CardRow{
			{Label: "縣市", Value: strVal(p.City)},
			{Label: "醫院", Value: strVal(p.Hospital), Wrap: true},
			{Label: "科別", Value: strOrDefault(p.Department)},
			{Label: "學歷", Value: strOrDefault(p.Education), Wrap: true},
			{Label: "資料來源", Value: sourceLabel(p.Source), Wrap: true},
		},
		SameDepartmentText: sameDepartmentSearchText(p),
	}
	if p.SourceURL.Valid && p.SourceURL.String != "" {
		c.SourceURL = p.SourceURL.String
		c.SourceURLText = "點此查看來源"
		if p.SourceCheckedAt.Valid {
			c.SourceURLText = fmt.Sprintf("點此查看來源（%s 查到）", p.SourceCheckedAt.Time.Format("2006-01-02"))
		}
	}
	return c
}

// sameDepartmentSearchText is what the button sends: a keyword search a user could
// type, in the doctor's city, for the hospital and the first department listed. It is
// "" when either is missing or the text is over LINE's limit.
func sameDepartmentSearchText(p database.MedicalPersonnel) string {
	depts := strings.FieldsFunc(p.Department.String, func(r rune) bool {
		return r == '、' || r == '/' || r == '／' || unicode.IsSpace(r)
	})
	hospital := strings.Fields(p.Hospital)
	if len(depts) == 0 || len(hospital) == 0 {
		return ""
	}
	words := append(append([]string{p.City}, hospital...), depts[0])
	text := strings.TrimSpace(strings.Join(words, " "))
	if utf8.RuneCountInString(text) > MaxMessageTextRunes {
		return ""
	}
	return text
}

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
			// The blog stopped updating in 2025-03; its hospitals are often out of date (OD-13).
			return "網路公開彙整（2025 年 3 月前）"
		case "public_report":
			return "民眾回報"
		case "hospital_directory":
			return "醫療院所官網"
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

	c := NewCard(p)
	var bodyContents []any
	for i, r := range c.Rows {
		var extra map[string]any
		if r.Wrap {
			extra = map[string]any{"wrap": true}
		}
		box := row(r.Label, r.Value, extra)
		if i > 0 {
			box["margin"] = "md"
		}
		bodyContents = append(bodyContents, box)
	}
	if c.SourceURL != "" {
		urlRow := row(SourceLinkLabel, c.SourceURLText, map[string]any{
			"wrap":  true,
			"color": "#0066CC",
			"action": map[string]any{
				"type":  "uri",
				"label": "查看來源",
				"uri":   c.SourceURL,
			},
		})
		urlRow["margin"] = "md"
		bodyContents = append(bodyContents, urlRow)
	}

	bubble := map[string]any{
		"type": "bubble",
		"header": map[string]any{
			"type":   "box",
			"layout": "vertical",
			"contents": []any{
				map[string]any{
					"type":   "text",
					"text":   c.Name,
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
	if text := c.SameDepartmentText; text != "" {
		bubble["footer"] = map[string]any{
			"type":   "box",
			"layout": "vertical",
			"contents": []any{map[string]any{
				"type":   "button",
				"style":  "link",
				"height": "sm",
				"action": map[string]any{"type": "message", "label": SameDepartmentLabel, "text": text},
			}},
		}
	}
	return bubble
}

// DoctorsFlexMessage builds the raw JSON for a flex message containing a carousel
// of doctor cards, or a "no results" bubble if the list is empty.
func DoctorsFlexMessage(personnel []database.MedicalPersonnel) (json.RawMessage, error) {
	var altText string
	var contents map[string]any

	if len(personnel) == 0 {
		altText = NoResultsText
		contents = map[string]any{
			"type": "bubble",
			"body": map[string]any{
				"type":   "box",
				"layout": "vertical",
				"contents": []any{
					map[string]any{
						"type":   "text",
						"text":   NoResultsText,
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
