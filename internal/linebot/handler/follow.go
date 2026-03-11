package handler

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/client"
	"github.com/dfgh012316/popofinder/internal/linebot/template"
	"github.com/dfgh012316/popofinder/internal/linebot/webhook"
)

// FollowHandler handles FollowEvent — sends the help message to new followers.
type FollowHandler struct{}

func (h *FollowHandler) Handle(ctx context.Context, event webhook.Event, c *client.Client, _ *database.Repository) error {
	slog.Info("[Follow]", "userId", event.Source.UserID)

	helpMsg, err := template.HelpFlexMessage()
	if err != nil {
		return err
	}
	return c.Reply(ctx, event.ReplyToken, []json.RawMessage{helpMsg})
}
