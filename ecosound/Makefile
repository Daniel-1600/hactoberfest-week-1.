SHELL := /bin/bash
RUFF ?= .venv/bin/ruff
# Local configuration is read by make and exported to both dev processes.
ifneq (,$(wildcard .env))
include .env
export
endif

.PHONY: setup setup-python setup-go setup-frontend dev-backend dev-frontend test test-go test-python test-frontend lint build verify-inference

setup: setup-python setup-go setup-frontend

setup-python:
	python3 -m venv .venv
	.venv/bin/python -m pip install -r inference/requirements.txt -r inference/requirements-dev.txt

setup-go:
	cd backend && go mod download

setup-frontend:
	cd frontend && npm ci

dev-backend:
	cd backend && go run ./cmd/server

dev-frontend:
	cd frontend && npm run dev

test: test-go test-python test-frontend

test-go:
	cd backend && go test ./...

test-python:
	python3 -m unittest discover -s inference -p 'test_*.py' -v

test-frontend:
	cd frontend && npm test

lint:
	cd backend && go vet ./...
	@test -z "$$(gofmt -l backend)" || (echo 'Run gofmt on the listed backend files:' && gofmt -l backend && exit 1)
	python3 -m compileall -q inference
	$(RUFF) check --no-cache inference
	cd frontend && npm run lint && npm run typecheck

build:
	mkdir -p bin
	cd backend && go build -buildvcs=false -o ../bin/ecosound-server ./cmd/server
	cd frontend && npm run build

# AUDIO must be a local decoded 1–60 second mono PCM WAV; this runs real inference.
verify-inference:
	@test -n "$(AUDIO)" || (echo 'Usage: make verify-inference AUDIO=/absolute/path/bird.wav' && exit 1)
	BIRDNET_APP_DATA="$$(pwd)/data/birdnet" .venv/bin/python inference/identify.py "$(AUDIO)"
