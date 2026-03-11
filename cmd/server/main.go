package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dfgh012316/popofinder/internal/config"
	"github.com/dfgh012316/popofinder/internal/database"
	lineclient "github.com/dfgh012316/popofinder/internal/linebot/client"
	"github.com/dfgh012316/popofinder/internal/linebot/handler"
	"github.com/dfgh012316/popofinder/internal/linebot/session"
	"github.com/dfgh012316/popofinder/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	db, err := database.Connect(cfg.DatabaseURL())
	if err != nil {
		slog.Error("connect database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	repo := database.NewRepository(db)
	store := session.NewStore()
	lineClient := lineclient.New(cfg.LineMessageChannelToken)
	dispatcher := handler.NewDispatcher(store, repo)

	srv := &http.Server{
		Addr:         ":8000",
		Handler:      server.New(cfg.LineMessageChannelSecret, lineClient, dispatcher, repo),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("starting server", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}
