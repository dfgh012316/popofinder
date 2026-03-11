package handler

import (
	"context"
	"log/slog"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/client"
	"github.com/dfgh012316/popofinder/internal/linebot/session"
	"github.com/dfgh012316/popofinder/internal/linebot/webhook"
)

// Handler processes a LINE webhook event.
type Handler interface {
	Handle(ctx context.Context, event webhook.Event, c *client.Client, repo *database.Repository) error
}

// Dispatcher routes events to their respective handlers.
type Dispatcher struct {
	handlers map[string]Handler
}

func NewDispatcher(store *session.Store, repo *database.Repository) *Dispatcher {
	d := &Dispatcher{handlers: make(map[string]Handler)}
	d.handlers["message"] = &MessageHandler{store: store}
	d.handlers["postback"] = &PostbackHandler{store: store}
	d.handlers["follow"] = &FollowHandler{}
	return d
}

// Dispatch routes the event to the appropriate handler. Unknown event types are ignored.
func (d *Dispatcher) Dispatch(ctx context.Context, event webhook.Event, c *client.Client, repo *database.Repository) {
	h, ok := d.handlers[event.Type]
	if !ok {
		return
	}
	if err := h.Handle(ctx, event, c, repo); err != nil {
		slog.Error("handler error", "type", event.Type, "user", event.Source.UserID, "err", err)
	}
}
