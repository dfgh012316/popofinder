package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/url"
	"strconv"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/client"
	"github.com/dfgh012316/popofinder/internal/linebot/search"
	"github.com/dfgh012316/popofinder/internal/linebot/session"
	"github.com/dfgh012316/popofinder/internal/linebot/webhook"
)

// PostbackHandler handles PostbackEvent — used for pagination.
type PostbackHandler struct {
	store *session.Store
}

func (h *PostbackHandler) Handle(ctx context.Context, event webhook.Event, c *client.Client, repo *database.Repository) error {
	if event.Postback == nil {
		return nil
	}

	params, err := url.ParseQuery(event.Postback.Data)
	if err != nil {
		return nil
	}

	if params.Get("action") != "next_page" {
		return nil
	}

	userID := event.Source.UserID
	slog.Info("[Postback] next_page", "userId", userID)

	state := h.store.Get(userID)
	if state == nil {
		expiredMsg, _ := json.Marshal(map[string]any{
			"type": "text",
			"text": "搜尋已過期，請重新搜尋",
		})
		return c.Reply(ctx, event.ReplyToken, []json.RawMessage{expiredMsg})
	}

	offset, _ := strconv.Atoi(params.Get("offset"))

	criteria := search.Criteria{
		SearchType: state.SearchType,
		SearchTerm: state.SearchTerm,
		City:       state.City,
	}

	personnel, stats, err := repo.Search(criteria, offset)
	if err != nil {
		return err
	}

	messages, err := buildSearchResponse(criteria, personnel, stats)
	if err != nil {
		return err
	}
	return c.Reply(ctx, event.ReplyToken, messages)
}
