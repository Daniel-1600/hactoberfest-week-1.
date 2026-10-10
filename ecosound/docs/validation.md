# Validation record

This document separates checks that can run without downloaded model assets from the end-to-end identification check.

## Automated checks

Run from the repository's `ecosound` directory:

```sh
python3 -m unittest discover -s inference -p 'test_*.py' -v
cd backend && go test ./...
```

On 2026-10-10, `make test` passed the full Go suite, all 12 Python boundary tests, and all 12 frontend workflow tests. `make lint` passed Go vet/formatting, Python syntax/Ruff, ESLint, and strict TypeScript checking. The Go race detector also passed. The smoke script passed `bash -n` syntax validation. These checks establish the local contracts, not real species identification.

The sandbox also refused a local socket connection (`curl` to `127.0.0.1:8080` returned “Operation not permitted”). Consequently, a running-server HTTP smoke check and browser walkthrough were not completed in this session. The Go HTTP tests use `httptest` and do not require a listening socket. Run the smoke script and browser check in an environment that allows localhost connections.

The Python tests construct objects with BirdNET's source-verified structured-array shape. They cover original label preservation, temporal rows and score precision, valid empty results, unprocessable inputs, missing fields, invalid scores/times, WAV validation, and CLI failure codes. They do not inject fake species into the running application or claim a successful model call.

The Go tests exercise HTTP behavior and process boundaries using temporary local data and fixture processes. They verify request validation, storage and quest invariants, and error handling without downloading BirdNET.

## Manual browser check

1. Start the backend and frontend using [README.md](../README.md), and open the local site.
2. Upload a supported 1–60 second bird recording. Check that the returned candidates have a common/scientific name, confidence, and time window, or that a genuine no-detection message appears.
3. Save one candidate. Check that it appears in the collection with playable audio and today's quest updates after saving.
4. Attempt another candidate from the same recording. Expect a conflict and no extra quest progress.
5. Confirm a saved discovery. Check the “Personally confirmed — not independently scientifically verified” wording and that quest counts stay unchanged.
6. Upload invalid bytes, an oversized file, and audio outside the duration bounds. Check that each yields a clear error without a saved discovery.

## Real-model status

At initial implementation, Python 3.14.7 and FFmpeg 8.1.2 were present, but installing `birdnet==1.1.1` and downloading the official soundscape failed because DNS resolution was unavailable in the sandbox. BirdNET was not installed there, and no real model output or species identification was observed. The adapter's method signature and structured-array fields were checked against the [official BirdNET source](https://github.com/birdnet-team/birdnet/blob/main/src/birdnet/acoustic/inference/core/prediction/prediction_result.py) and [3.0 model implementation](https://github.com/birdnet-team/birdnet/blob/main/src/birdnet/acoustic/models/v3_0/model.py). A connected environment still needs to complete the install, first model download, and manual bird-audio check above before claiming end-to-end identification works.

## Checks observed during implementation (2026-10-10)

- **AI feasibility:** blocked. Package installation failed on DNS resolution; a valid three-second decoded WAV reached the import boundary and exited 1 with `ModuleNotFoundError: No module named 'birdnet'`, emitting no success JSON. The official 1.1.1 source supplied the adapter schema and signature. No real model prediction has been observed.
- **Backend:** `go test ./...` and `go vet ./...` passed. HTTP handler tests cover identification persistence, discovery save/list/confirm, unknown IDs and duplicate conflicts, immutable predictions, foreign keys, database restart, no-detection responses, upload bounds, CORS, and inference failures. Real FFmpeg conversion tests passed for WAV, WebM, M4A, Ogg, MP3, and FLAC, with invalid audio, length bounds, and temporary cleanup checked. Subprocess tests cover strict JSON parsing and timeout cancellation of child workers.
- **Python boundary:** 12 standard-library unittest cases passed, including successful/failing exit contracts and native/Python diagnostic isolation. All successes in automated inference-interface tests are explicitly mocked; the running application has no fixture mode.
- **Server binary:** compiled with `go build -buildvcs=false`. This flag avoids VCS stamping against the environment's read-only Git metadata placeholder and is included in `make build`.
- **Frontend:** all 12 Vitest workflow tests, ESLint, and TypeScript passed. Tests cover microphone permissions, unsupported capture, stop/playback, automatic duration limits, late-permission cleanup, identification and no-detection rendering, save/duplicate handling, server-derived quest refresh/completion, and personal confirmation. Browser and API behavior is mocked in these tests.
- **Dependency reproducibility:** `npm ci --offline --cache=/tmp/ecosound-npm-cache` passed using the committed ordinary registry lockfile and cached Linux packages. The project `.npmrc` uses `legacy-peer-deps=true` to avoid npm 10.9.7's optional test-tool peer resolver error. This is an install workaround, not an inference substitute.
- **Production build:** `make build` passed, producing the Go executable and compiled Next.js app. Next.js prerenders the page shell; interactive recording, identification, collection, and quest data use the live Go API at runtime. The normal frontend build also exited successfully, with a non-fatal repair diagnostic for uncached optional SWC platform metadata. The final combined build used `NEXT_TELEMETRY_DISABLED=1 NEXT_IGNORE_INCORRECT_LOCKFILE=1` to skip that offline repair on Linux. See [frontend/README.md](../frontend/README.md) for the platform-cache limitation.
- **Live integration:** blocked. `go run ./cmd/server` compiled, opened/migrated SQLite, then failed to bind `127.0.0.1:8080` with `socket: operation not permitted`. The smoke script passed shell syntax validation but was not run against a listening service.

Go checks used `GOCACHE=/tmp/ecosound-go-cache` and cached module dependencies with `GOPROXY=off GOSUMDB=off` because the default cache filesystem is read-only. An invocation without that cache override failed on the read-only cache; rerunning with the override passed. The combined lint command used `RUFF=ruff` for the preinstalled executable because Python package installation was blocked. These are environment-specific overrides, not requirements for a normal local installation.

## Remaining acceptance checks

The code and automated checks are complete, but the application's end-to-end definition of done is not yet verified. A connected machine that permits local servers must install BirdNET, download its weights, identify authentic bird audio, and run the browser recording → identify → save → collection → quest-completion loop. Physical microphone behavior, visual layout in a real browser, and the 90-second demo timing remain unverified. No fixture predictions are available in the production application.
