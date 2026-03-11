package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/dfgh012316/popofinder/internal/database"
	"github.com/dfgh012316/popofinder/internal/linebot/client"
	"github.com/dfgh012316/popofinder/internal/linebot/handler"
	"github.com/dfgh012316/popofinder/internal/linebot/webhook"
)

type Server struct {
	router     *chi.Mux
	lineClient *client.Client
	dispatcher *handler.Dispatcher
	secret     string
	repo       *database.Repository
}

func New(secret string, lineClient *client.Client, dispatcher *handler.Dispatcher, repo *database.Repository) *Server {
	s := &Server{
		router:     chi.NewRouter(),
		lineClient: lineClient,
		dispatcher: dispatcher,
		secret:     secret,
		repo:       repo,
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.RequestID)

	s.router.Get("/health", s.handleHealth)

	s.router.Route("/api", func(r chi.Router) {
		r.Route("/linebot", func(r chi.Router) {
			r.Post("/callback", s.handleCallback)
		})
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "cannot read body", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Line-Signature")
	if !webhook.ValidateSignature(s.secret, sig, body) {
		slog.Error("invalid signature")
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	wbody, err := webhook.ParseBody(body)
	if err != nil {
		slog.Error("parse webhook body", "err", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	for _, event := range wbody.Events {
		s.dispatcher.Dispatch(r.Context(), event, s.lineClient, s.repo)
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}
