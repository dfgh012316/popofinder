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

// SourceLinkLabel is the button that opens where the data came from.
const SourceLinkLabel = "查看來源"

// The headings of the card's 學歷, 專長 and 經歷 (OD-18).
const (
	EducationLabel  = "學歷"
	SpecialtyLabel  = "專長"
	ExperienceLabel = "經歷"
)

// ExperienceShown is how many 經歷 lines a card shows before the rest is folded
// (web) or left out (LINE, where the message has a size limit).
const ExperienceShown = 3

// NoResultsText is shown when a search finds nothing.
const NoResultsText = "找不到相關資料"

// Card is what a doctor card shows. The LINE bubble and the web page (OD-17) both
// render it, so the two show the same fields.
type Card struct {
	Name                       string
	Hospital, Department, City string   // "" when missing; the card leaves them out
	Education                  []string // one entry per line; 「未提供」 when there is none
	Specialty, Experience      []string // 專長 and 經歷 from the hospital's page, one entry per line; nil when there is none (OD-18)
	Source                     string   // where the data came from, e.g. 醫療院所官網
	SourceURL                  string   // "" when there is no source link
	CheckedOn                  string   // when the source page was last read, "2006-01-02"; "" without a source link
	SameDepartmentText         string   // what 「同院同科醫師」 searches; "" when there is no button
}

// SourceNote is the card's last line: the source, and when its page was read.
func (c Card) SourceNote() string {
	if c.CheckedOn != "" {
		return c.Source + "，" + c.CheckedOn + " 查到"
	}
	return c.Source
}

// NewCard builds the card for one doctor.
func NewCard(p database.MedicalPersonnel) Card {
	c := Card{
		Name:               p.Name,
		Hospital:           strings.TrimSpace(p.Hospital),
		Department:         strings.TrimSpace(p.Department.String),
		City:               strings.TrimSpace(p.City),
		Education:          lines(p.Education.String),
		Specialty:          lines(p.Specialty.String),
		Experience:         lines(p.Experience.String),
		Source:             sourceLabel(p.Source),
		SameDepartmentText: sameDepartmentSearchText(p),
	}
	if len(c.Education) == 0 {
		c.Education = []string{"未提供"}
	}
	if p.SourceURL.Valid && p.SourceURL.String != "" {
		c.SourceURL = p.SourceURL.String
		if p.SourceCheckedAt.Valid {
			c.CheckedOn = p.SourceCheckedAt.Time.Format("2006-01-02")
		}
	}
	return c
}

// ExperienceHead is the first ExperienceShown 經歷 lines, the ones always shown.
func (c Card) ExperienceHead() []string {
	return c.Experience[:min(len(c.Experience), ExperienceShown)]
}

// ExperienceRest is the 經歷 lines after ExperienceHead; nil when there are none.
func (c Card) ExperienceRest() []string {
	if len(c.Experience) <= ExperienceShown {
		return nil
	}
	return c.Experience[ExperienceShown:]
}

// lines splits a stored list (the crawler joins lines with 「；」, OD-10) into its
// non-blank lines.
func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "；") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
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

// The LINE card's colors, the same ink and blue as the web page's light theme.
const (
	inkColor       = "#1B2436"
	ink2Color      = "#3B4458"
	mutedColor     = "#646C7E"
	linkColor      = "#1F4BB8"
	educationColor = "#EEF3FD"
)

// How much of the 專長 and of each 經歷 line a LINE card shows.
const (
	lineSpecialtyRunes  = 80
	lineExperienceRunes = 40
)

// cut shortens s to n runes, ending with 「…」 when it was longer.
func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// doctorBubble lays a card out as a LINE bubble: the name, the hospital, the
// department and city, the 學歷 lines in a tinted block, the 專長 and the first
// 經歷 lines, then the source; the
// buttons open the source and search the same department.
func doctorBubble(p database.MedicalPersonnel) map[string]any {
	c := NewCard(p)
	text := func(s string, props map[string]any) map[string]any {
		n := map[string]any{"type": "text", "text": s, "wrap": true}
		for k, v := range props {
			n[k] = v
		}
		return n
	}

	body := []any{text(c.Name, map[string]any{"size": "xl", "weight": "bold", "color": inkColor})}
	if c.Hospital != "" {
		body = append(body, text(c.Hospital, map[string]any{"size": "sm", "weight": "bold", "color": ink2Color, "margin": "sm"}))
	}
	var where []string
	for _, w := range []string{c.Department, c.City} {
		if w != "" {
			where = append(where, w)
		}
	}
	if len(where) > 0 {
		body = append(body, text(strings.Join(where, "　"), map[string]any{"size": "xs", "color": mutedColor}))
	}
	// One text per field with the lines joined by \n: a text node per line would
	// add about 80 bytes each, and a page of cards has a size limit.
	education := []any{
		text(EducationLabel, map[string]any{"size": "xxs", "color": mutedColor}),
		text(strings.Join(c.Education, "\n"), map[string]any{"size": "sm", "weight": "bold", "color": inkColor}),
	}
	body = append(body, map[string]any{
		"type": "box", "layout": "vertical", "contents": education, "spacing": "xs",
		"backgroundColor": educationColor, "cornerRadius": "md", "paddingAll": "md", "margin": "lg",
	})
	// 專長 and 經歷 are cut short: a carousel of ten cards must stay under LINE's
	// size limit, and 查看來源 has the whole page.
	detail := func(label string, ls []string) {
		if len(ls) == 0 {
			return
		}
		body = append(body,
			text(label, map[string]any{"size": "xxs", "color": mutedColor, "margin": "lg"}),
			text(strings.Join(ls, "\n"), map[string]any{"size": "xs", "color": ink2Color}))
	}
	if len(c.Specialty) > 0 {
		detail(SpecialtyLabel, []string{cut(strings.Join(c.Specialty, "；"), lineSpecialtyRunes)})
	}
	var experience []string
	for _, l := range c.ExperienceHead() {
		experience = append(experience, cut(l, lineExperienceRunes))
	}
	detail(ExperienceLabel, experience)
	if rest := len(c.ExperienceRest()); rest > 0 {
		// Muted, so it does not read as one more line of 經歷.
		body = append(body, text(fmt.Sprintf("另有 %d 項，按「%s」看全部", rest, SourceLinkLabel), map[string]any{"size": "xxs", "color": mutedColor}))
	}
	body = append(body, text(c.SourceNote(), map[string]any{"size": "xxs", "color": mutedColor, "margin": "lg"}))

	bubble := map[string]any{
		"type": "bubble",
		"body": map[string]any{"type": "box", "layout": "vertical", "contents": body},
	}
	var buttons []any
	button := func(action map[string]any) map[string]any {
		return map[string]any{"type": "button", "style": "link", "height": "sm", "color": linkColor, "action": action}
	}
	if c.SourceURL != "" {
		buttons = append(buttons, button(map[string]any{"type": "uri", "label": SourceLinkLabel, "uri": c.SourceURL}))
	}
	if t := c.SameDepartmentText; t != "" {
		buttons = append(buttons, button(map[string]any{"type": "message", "label": SameDepartmentLabel, "text": t}))
	}
	if len(buttons) > 0 {
		bubble["footer"] = map[string]any{"type": "box", "layout": "horizontal", "spacing": "sm", "contents": buttons}
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
