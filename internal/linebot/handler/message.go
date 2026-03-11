package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/client"
	"github.com/dfgh012316/popofinder/internal/linebot/search"
	"github.com/dfgh012316/popofinder/internal/linebot/session"
	"github.com/dfgh012316/popofinder/internal/linebot/template"
	"github.com/dfgh012316/popofinder/internal/linebot/webhook"
)

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
	if lower == "幫助" || lower == "help" {
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
		search.TypeName:       "醫師",
		search.TypeHospital:   "醫院",
		search.TypeDepartment: "科別",
	}[criteria.SearchType]

	start := stats.CurrentPage*10 - 9
	end := stats.CurrentPage * 10
	if end > stats.TotalCount {
		end = stats.TotalCount
	}

	return fmt.Sprintf("查詢%s%s「%s」\n共有 %d 筆符合的結果\n目前顯示第 %d - %d 筆",
		location, typeText, criteria.SearchTerm,
		stats.TotalCount,
		start, end,
	)
}
