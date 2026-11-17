package handler

import (
	"context"
	"log/slog"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/client"
	"github.com/dfgh012316/popofinder/internal/linebot/template"
	"github.com/dfgh012316/popofinder/internal/linebot/webhook"
)

// FollowHandler handles FollowEvent — sends the help and support messages to new followers.
type FollowHandler struct{}

func (h *FollowHandler) Handle(ctx context.Context, event webhook.Event, c *client.Client, _ *database.Repository) error {
	slog.Info("[Follow]", "userId", event.Source.UserID)

	msgs, err := template.HelpMessages()
	if err != nil {
		return err
	}
	return c.Reply(ctx, event.ReplyToken, msgs)
}
