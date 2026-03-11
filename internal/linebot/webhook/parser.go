package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// ValidateSignature validates a LINE webhook X-Line-Signature header.
func ValidateSignature(channelSecret, signature string, body []byte) bool {
	mac := hmac.New(sha256.New, []byte(channelSecret))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ParseBody deserialises the raw webhook body into a Body struct.
func ParseBody(body []byte) (Body, error) {
	var b Body
	if err := json.Unmarshal(body, &b); err != nil {
		return Body{}, fmt.Errorf("webhook: parse: %w", err)
	}
	return b, nil
}
