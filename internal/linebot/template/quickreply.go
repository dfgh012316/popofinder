package template

import "encoding/json"

// MaxMessageTextRunes is LINE's limit for the text a message action sends.
const MaxMessageTextRunes = 300

// QuickReplyAction is a message-type quick reply button: tapping it sends Text as the user.
// LINE limits Label to 20 characters and Text to 300.
type QuickReplyAction struct {
	Label string
	Text  string
}

// WithQuickReply returns msg with a quickReply attached. LINE only shows the quick reply
// of the last message in a reply, so callers attach it to the final message.
func WithQuickReply(msg json.RawMessage, actions []QuickReplyAction) (json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(msg, &m); err != nil {
		return nil, err
	}
	items := make([]any, len(actions))
	for i, a := range actions {
		items[i] = map[string]any{
			"type":   "action",
			"action": map[string]any{"type": "message", "label": a.Label, "text": a.Text},
		}
	}
	qr, err := json.Marshal(map[string]any{"items": items})
	if err != nil {
		return nil, err
	}
	m["quickReply"] = qr
	return json.Marshal(m)
}
