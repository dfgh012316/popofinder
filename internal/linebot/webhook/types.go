package webhook

import "encoding/json"

// Body is the top-level LINE webhook payload.
type Body struct {
	Destination string  `json:"destination"`
	Events      []Event `json:"events"`
}

// Event represents a single LINE event. The Message field is only present for
// message events; Postback is only present for postback events.
type Event struct {
	Type       string          `json:"type"`
	ReplyToken string          `json:"replyToken"`
	Source     Source          `json:"source"`
	Timestamp  int64           `json:"timestamp"`
	Message    json.RawMessage `json:"message,omitempty"`
	Postback   *PostbackData   `json:"postback,omitempty"`
}

// Source holds information about the event source.
type Source struct {
	Type   string `json:"type"`
	UserID string `json:"userId"`
}

// TextMessage is the parsed content of a text message event.
type TextMessage struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
}

// PostbackData holds the data string sent by a postback action.
type PostbackData struct {
	Data string `json:"data"`
}
