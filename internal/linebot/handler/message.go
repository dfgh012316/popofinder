package handler

import (
	"context"
	"encoding/json"
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

	// Hospital list command (OD-12)
	if text == template.HospitalListCommand {
		hospitals, err := repo.DirectoryHospitals(ctx)
		if err != nil {
			return err
		}
		listMsg, err := template.HospitalListFlexMessage(hospitals)
		if err != nil {
			return err
		}
		return c.Reply(ctx, event.ReplyToken, []json.RawMessage{listMsg})
	}

	// How to add, correct or take down data (OD-15)
	if text == template.ReportCommand || lower == "report" {
		reportMsg, err := template.ReportTextMessage()
		if err != nil {
			return err
		}
		return c.Reply(ctx, event.ReplyToken, []json.RawMessage{reportMsg})
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

	if err := repo.LogSearch(ctx, criteria, stats.TotalCount, database.ChannelLINE); err != nil {
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
		"text": template.SearchSummary(criteria, stats),
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
		nextOffset := stats.CurrentPage * database.PageSize
		nextPageMsg, err := template.NextPageFlexMessage(stats.CurrentPage, stats.TotalPages, nextOffset)
		if err != nil {
			return nil, err
		}
		messages = append(messages, nextPageMsg)
	}

	return messages, nil
}

// zeroResultActions suggests next steps when a search finds nothing: widen a narrowed
// search (city or @ type) to an all-field nationwide search, and always offer help.
func zeroResultActions(criteria search.Criteria) []template.QuickReplyAction {
	var actions []template.QuickReplyAction
	n := utf8.RuneCountInString(criteria.SearchTerm)
	narrowed := criteria.City != nil || criteria.SearchType != search.TypeAll
	if narrowed && n > 0 && n <= template.MaxMessageTextRunes {
		actions = append(actions, template.QuickReplyAction{Label: "擴大搜尋", Text: criteria.SearchTerm})
	}
	return append(actions, template.QuickReplyAction{Label: "使用說明", Text: helpCommand})
}
