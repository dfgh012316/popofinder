package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubPinger struct{ err error }

func (s stubPinger) Ping(ctx context.Context) error { return s.err }

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func TestHandleHealth(t *testing.T) {
	s := &Server{}
	rec := httptest.NewRecorder()
	s.handleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health expected 200, got %d", rec.Code)
	}
	if body := decode(t, rec); body["status"] != "ok" {
		t.Fatalf("unexpected health body: %v", body)
	}
}

func TestHandleReadyOK(t *testing.T) {
	s := &Server{ready: stubPinger{err: nil}}
	rec := httptest.NewRecorder()
	s.handleReady(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("readyz expected 200, got %d", rec.Code)
	}
	body := decode(t, rec)
	checks, _ := body["checks"].(map[string]any)
	if body["status"] != "ok" || checks["db"] != "ok" {
		t.Fatalf("unexpected readyz body: %v", body)
	}
}

func TestHandleReadyUnhealthy(t *testing.T) {
	s := &Server{ready: stubPinger{err: errors.New("db down")}}
	rec := httptest.NewRecorder()
	s.handleReady(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("readyz on dead DB expected 503, got %d", rec.Code)
	}
	body := decode(t, rec)
	checks, _ := body["checks"].(map[string]any)
	if body["status"] != "unhealthy" || checks["db"] != "error" {
		t.Fatalf("unexpected unhealthy body: %v", body)
	}
}

func TestHandleVersion(t *testing.T) {
	s := &Server{version: "test-version"}
	rec := httptest.NewRecorder()
	s.handleVersion(rec, httptest.NewRequest(http.MethodGet, "/version", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("version expected 200, got %d", rec.Code)
	}
	if body := decode(t, rec); body["version"] != "test-version" {
		t.Fatalf("unexpected version body: %v", body)
	}
}
