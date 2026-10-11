package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"ecosound/backend/internal/ai/llm"
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
	var identifier inference.Identifier
	switch env("INFERENCE_PROVIDER", "local") {
	case "local":
		python := env("PYTHON_EXECUTABLE", "../.venv/bin/python")
		identifier = inference.Process{Python: python, Script: env("INFERENCE_SCRIPT", "../inference/identify.py"), Timeout: timeout}
	case "modal":
		identifier = inference.Modal{Endpoint: os.Getenv("BIRDNET_MODAL_URL"), TokenID: os.Getenv("MODAL_PROXY_TOKEN_ID"), TokenSecret: os.Getenv("MODAL_PROXY_TOKEN_SECRET"), Timeout: timeout}
	default:
		return errors.New("INFERENCE_PROVIDER must be local or modal")
	}
	qwenTimeout, err := time.ParseDuration(env("QWEN_TIMEOUT_SECONDS", "60") + "s")
	if err != nil || qwenTimeout <= 0 || qwenTimeout > 5*time.Minute {
		return errors.New("QWEN_TIMEOUT_SECONDS must be between 1 and 300")
	}
	qwenEnabled, err := strconv.ParseBool(env("QWEN_ENABLED", "false"))
	if err != nil {
		return errors.New("QWEN_ENABLED must be true or false")
	}
	natureLMEnabled, err := strconv.ParseBool(env("NATURELM_ENABLED", "false"))
	if err != nil {
		return errors.New("NATURELM_ENABLED must be true or false")
	}
	natureTimeout, err := time.ParseDuration(env("NATURELM_TIMEOUT_SECONDS", "120") + "s")
	if err != nil || natureTimeout <= 0 || natureTimeout > 5*time.Minute {
		return errors.New("NATURELM_TIMEOUT_SECONDS must be between 1 and 300")
	}
	qwenProvider := env("QWEN_PROVIDER", "local")
	qwenEndpoint := env("QWEN_ENDPOINT", "http://127.0.0.1:11434/v1/chat/completions")
	qwenTokenID, qwenTokenSecret := "", ""
	switch qwenProvider {
	case "local":
	case "modal":
		qwenEndpoint = strings.TrimRight(os.Getenv("QWEN_MODAL_URL"), "/") + "/v1/chat/completions"
		qwenTokenID, qwenTokenSecret = os.Getenv("QWEN_MODAL_PROXY_TOKEN_ID"), os.Getenv("QWEN_MODAL_PROXY_TOKEN_SECRET")
		if qwenEnabled && (qwenEndpoint == "/v1/chat/completions" || qwenTokenID == "" || qwenTokenSecret == "") {
			return errors.New("QWEN_MODAL_URL and QWEN_MODAL_PROXY_TOKEN_ID/SECRET are required when Qwen is enabled with QWEN_PROVIDER=modal")
		}
	default:
		return errors.New("QWEN_PROVIDER must be local or modal")
	}
	qwen := llm.Client{Endpoint: qwenEndpoint, Model: env("QWEN_MODEL_PATH", "qwen3:1.7b"), Timeout: qwenTimeout, ProxyTokenID: qwenTokenID, ProxyTokenKey: qwenTokenSecret}
	mapEnabled, err := strconv.ParseBool(env("PUBLIC_MAP_ENABLED", "true"))
	if err != nil {
		return errors.New("PUBLIC_MAP_ENABLED must be true or false")
	}
	natureLM := inference.NatureLMProvider{Endpoint: env("NATURELM_ENDPOINT", "http://127.0.0.1:8091"), Enabled: natureLMEnabled, Timeout: natureTimeout}
	apiServer, err := api.New(api.Config{DB: database, Storage: audio, Identifier: identifier, QuestTimezone: zone, Origin: env("FRONTEND_ORIGIN", "http://localhost:3000"), Qwen: qwen, QwenEnabled: qwenEnabled, NatureLMEnabled: natureLMEnabled, NatureLM: natureLM, PublicMapEnabled: mapEnabled, ObservationLocationPrecision: env("OBSERVATION_LOCATION_PRECISION", "coarse")})
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
