package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

const replyURL = "https://api.line.me/v2/bot/message/reply"

// Client sends messages via the LINE Messaging API.
type Client struct {
	httpClient  *http.Client
	accessToken string
}

func New(accessToken string) *Client {
	return &Client{
		httpClient:  &http.Client{},
		accessToken: accessToken,
	}
}

type replyRequest struct {
	ReplyToken string            `json:"replyToken"`
	Messages   []json.RawMessage `json:"messages"`
}

// Reply sends one or more messages in reply to a LINE event.
func (c *Client) Reply(ctx context.Context, replyToken string, messages []json.RawMessage) error {
	payload, err := json.Marshal(replyRequest{
		ReplyToken: replyToken,
		Messages:   messages,
	})
	if err != nil {
		return fmt.Errorf("client: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, replyURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("client: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("client: do: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("client: unexpected status %d", resp.StatusCode)
	}
	return nil
}
