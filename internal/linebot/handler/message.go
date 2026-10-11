package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/client"
	"github.com/dfgh012316/popofinder/internal/linebot/search"
	"github.com/dfgh012316/popofinder/internal/linebot/session"
	"github.com/dfgh012316/popofinder/internal/linebot/template"
	"github.com/dfgh012316/popofinder/internal/linebot/webhook"
)

// helpCommand is the text that shows the help card; quick replies send it too.
const helpCommand = "幫助"

// maxQuickReplyTextRunes is LINE's limit for a message action's text.
const maxQuickReplyTextRunes = 300

// MessageHandler handles text MessageEvent.
type MessageHandler struct {
	store *session.Store
}

func (h *MessageHandler) Handle(ctx context.Context, event webhook.Event, c *client.Client, repo *database.Repository) error {
	// Parse text message
	var msg webhook.TextMessage
	if err := json.Unmarshal(event.Message, &msg); err != nil || msg.Type != "text" {
		return nil // ignore non-text messages
	}

	userID := event.Source.UserID
	text := strings.TrimSpace(msg.Text)

	slog.Info("[Message]", "userId", userID, "text", text)

	// Help command
	lower := strings.ToLower(text)
	if lower == helpCommand || lower == "help" {
		helpMsg, err := template.HelpFlexMessage()
		if err != nil {
			return err
		}
		return c.Reply(ctx, event.ReplyToken, []json.RawMessage{helpMsg})
	}

	// Ignore report command
	if lower == "回報" || lower == "report" {
		return nil
	}

	// Parse and execute search
	criteria := search.ParseCriteria(text)
	h.store.Set(userID, session.State{
		SearchTerm: criteria.SearchTerm,
		City:       criteria.City,
		SearchType: criteria.SearchType,
	})

	personnel, stats, err := repo.Search(criteria, 0)
	if err != nil {
		return err
	}

	if err := repo.LogSearch(ctx, criteria, stats.TotalCount); err != nil {
		slog.Error("log search", "err", err)
	}

	messages, err := buildSearchResponse(criteria, personnel, stats)
	if err != nil {
		return err
	}
	return c.Reply(ctx, event.ReplyToken, messages)
}

func buildSearchResponse(criteria search.Criteria, personnel []database.MedicalPersonnel, stats database.SearchStats) ([]json.RawMessage, error) {
	var messages []json.RawMessage

	// Summary text message
	summaryJSON, err := json.Marshal(map[string]any{
		"type": "text",
		"text": formatSummary(criteria, stats),
	})
	if err != nil {
		return nil, err
	}
	messages = append(messages, summaryJSON)

	// Flex message with doctor cards
	flexMsg, err := template.DoctorsFlexMessage(personnel)
	if err != nil {
		return nil, err
	}
	if stats.TotalCount == 0 {
		flexMsg, err = template.WithQuickReply(flexMsg, zeroResultActions(criteria))
		if err != nil {
			return nil, err
		}
	}
	messages = append(messages, flexMsg)

	// "Next page" button if there are more results
	if stats.HasMore {
		nextOffset := stats.CurrentPage * 10
		nextPageMsg, err := template.NextPageFlexMessage(stats.CurrentPage, stats.TotalPages, nextOffset)
		if err != nil {
			return nil, err
		}
		messages = append(messages, nextPageMsg)
	}

	return messages, nil
}

func formatSummary(criteria search.Criteria, stats database.SearchStats) string {
	var location string
	if criteria.City != nil {
		location = "在" + *criteria.City
	} else {
		location = "全台"
	}

	typeText := map[search.SearchType]string{
		search.TypeAll:        "",
		search.TypeHospital:   "醫院",
		search.TypeDepartment: "科別",
	}[criteria.SearchType]

	header := fmt.Sprintf("查詢%s%s「%s」", location, typeText, criteria.SearchTerm)
	if stats.TotalCount == 0 {
		return header + "\n共有 0 筆符合的結果"
	}

	countLine := fmt.Sprintf("共有 %d 筆符合的結果", stats.TotalCount)
	if criteria.SearchType == search.TypeAll {
		var groups []string
		for _, g := range []struct {
			label string
			n     int
		}{
			{"姓名", stats.MatchCounts.Name},
			{"醫院", stats.MatchCounts.Hospital},
			{"科別", stats.MatchCounts.Department},
		} {
			if g.n > 0 {
				groups = append(groups, fmt.Sprintf("%s %d", g.label, g.n))
			}
		}
		countLine += "（" + strings.Join(groups, "、") + "）"
	}

	start := stats.CurrentPage*10 - 9
	end := stats.CurrentPage * 10
	if end > stats.TotalCount {
		end = stats.TotalCount
	}
	return fmt.Sprintf("%s\n%s\n目前顯示第 %d - %d 筆", header, countLine, start, end)
}

// zeroResultActions suggests next steps when a search finds nothing: widen a narrowed
// search (city or @ type) to an all-field nationwide search, and always offer help.
func zeroResultActions(criteria search.Criteria) []template.QuickReplyAction {
	var actions []template.QuickReplyAction
	n := utf8.RuneCountInString(criteria.SearchTerm)
	narrowed := criteria.City != nil || criteria.SearchType != search.TypeAll
	if narrowed && n > 0 && n <= maxQuickReplyTextRunes {
		actions = append(actions, template.QuickReplyAction{Label: "擴大搜尋", Text: criteria.SearchTerm})
	}
	return append(actions, template.QuickReplyAction{Label: "使用說明", Text: helpCommand})
}
