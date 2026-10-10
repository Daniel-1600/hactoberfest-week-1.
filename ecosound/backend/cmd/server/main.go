package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"ecosound/backend/internal/api"
	"ecosound/backend/internal/db"
	"ecosound/backend/internal/inference"
	"ecosound/backend/internal/storage"
)

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func run() error {
	zone, err := time.LoadLocation(env("QUEST_TIMEZONE", "Africa/Nairobi"))
	if err != nil {
		return err
	}
	timeout, err := time.ParseDuration(env("INFERENCE_TIMEOUT", "180s"))
	if err != nil || timeout <= 0 || timeout > 210*time.Second {
		return errors.New("INFERENCE_TIMEOUT must be greater than zero and at most 210s")
	}
	port, err := strconv.Atoi(env("BACKEND_PORT", "8080"))
	if err != nil || port < 1 || port > 65535 {
		return errors.New("BACKEND_PORT must be a valid port")
	}
	database, err := db.Open(env("DATABASE_PATH", "../data/ecosound.sqlite"))
	if err != nil {
		return err
	}
	defer database.Close()
	audio, err := storage.New(env("AUDIO_STORAGE_DIR", "../data/audio"))
	if err != nil {
		return err
	}
	python := env("PYTHON_EXECUTABLE", "../.venv/bin/python")
	identifier := inference.Process{Python: python, Script: env("INFERENCE_SCRIPT", "../inference/identify.py"), Timeout: timeout}
	apiServer, err := api.New(api.Config{DB: database, Storage: audio, Identifier: identifier, QuestTimezone: zone, Origin: env("FRONTEND_ORIGIN", "http://localhost:3000")})
	if err != nil {
		return err
	}
	// Bind locally by default: the MVP has no authentication and contains private audio.
	server := &http.Server{Addr: env("BACKEND_HOST", "127.0.0.1") + ":" + strconv.Itoa(port), Handler: apiServer.Handler(), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 45 * time.Second, WriteTimeout: 250 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() {
		slog.Info("EcoSound API listening", "address", server.Addr, "quest_timezone", zone.String())
		done <- server.ListenAndServe()
	}()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return err
		}
	}
	return nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if err := run(); err != nil {
		slog.Error("EcoSound failed to start", "error", err)
		os.Exit(1)
	}
}
