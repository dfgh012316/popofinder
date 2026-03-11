package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func sign(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestValidateSignature(t *testing.T) {
	secret := "test-secret"
	body := []byte(`{"destination":"test","events":[]}`)

	valid := sign(secret, body)
	assert.True(t, ValidateSignature(secret, valid, body))
	assert.False(t, ValidateSignature(secret, "bad-sig", body))
	assert.False(t, ValidateSignature(secret, valid, []byte("tampered")))
}

func TestParseBody(t *testing.T) {
	body := []byte(`{"destination":"Uabc","events":[{"type":"follow","replyToken":"tok","source":{"type":"user","userId":"U123"},"timestamp":1234567890}]}`)
	wb, err := ParseBody(body)
	assert.NoError(t, err)
	assert.Equal(t, "Uabc", wb.Destination)
	assert.Len(t, wb.Events, 1)
	assert.Equal(t, "follow", wb.Events[0].Type)
	assert.Equal(t, "U123", wb.Events[0].Source.UserID)
}
