package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/kyleterry/booksmk/internal/server"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	srv, err := server.New(server.Config{
		Addr:        envOrDefault("BOOKSMK_ADDR", ":8080"),
		DatabaseURL: mustEnv(logger, "BOOKSMK_DATABASE_URL"),
		Logger:      logger,
	})
	if err != nil {
		logger.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := srv.Run(ctx); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func mustEnv(logger *slog.Logger, key string) string {
	v := os.Getenv(key)
	if v == "" {
		logger.Error("required environment variable not set", "key", key)
		os.Exit(1)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
