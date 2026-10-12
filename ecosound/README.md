# EcoSound — Turn Outdoor Exploration Into Wildlife Discovery

Record a short bird sound, review BirdNET 3.0 candidate identifications, and save one choice with replayable audio in a personal collection. The Morning Bird Quest tracks three distinct saved calls and two distinct recognized species during the configured local day. Model scores are suggestions, not guaranteed identifications.

EcoSound is a local, single-user MVP. It has no accounts or authentication. Keep the API bound to localhost unless everyone who can reach it should also be able to see the collection and hear its recordings.

## Requirements

For a hosted hackathon demo, use [the Render Blueprint](render.yaml) and follow
[the deployment guide](docs/render-deployment.md). It uses the existing Modal
models, two Starter services, and persistent storage for SQLite and audio.

- Go 1.25 or newer with CGO and a C compiler (SQLite driver)
- Python 3.11 or newer and pip, with network access for installing `birdnet==1.1.1` and downloading BirdNET model assets on first use
- Node.js 22.12 or newer and npm for the frontend and its development tools (tested with 22.22.2)
- FFmpeg and FFprobe on `PATH`

## Run locally

For the Makefile workflow, copy `.env.example` to `.env`, adjust paths if needed, then run `make setup`. Start `make dev-backend` and `make dev-frontend` in separate terminals. The direct commands below use the defaults. When running them directly, export any customized variables first; only the Makefile loads the root `.env` file automatically.

From the `ecosound` directory, install Python dependencies:

```sh
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -r inference/requirements.txt
```

In one terminal, start the Go API from `ecosound/backend`:

```sh
cd backend
go run ./cmd/server
```

The API defaults to `http://127.0.0.1:8080`, stores SQLite and private normalized WAV files under `ecosound/data/`, and calls `../.venv/bin/python ../inference/identify.py`. BirdNET assets can be kept under `ecosound/data/birdnet` by setting `BIRDNET_APP_DATA=../data/birdnet` for the backend, as in `.env.example`. `GET /health` checks the server and database; its `"inference":"configured"` value does not mean the model has downloaded successfully.

In another terminal, start the frontend from `ecosound/frontend`:

```sh
cd frontend
npm ci
npm run dev
```

Open `http://localhost:3000`. Allow microphone access for live recording, or upload a 1–60 second WAV, WebM, MP4/M4A, Ogg, MP3, or FLAC file. Uploads are capped at 15 MiB. The first real identification may need time to download model assets. If that download cannot complete, the API returns a model-unavailable error; it never substitutes sample wildlife results.

The default frontend API URL is `http://localhost:8080` and the default allowed frontend origin is `http://localhost:3000`. If you change either, set `NEXT_PUBLIC_API_URL` for the frontend and `FRONTEND_ORIGIN` for the backend to matching origins before starting them.

For a phone on the same network, the layout can be opened in its browser, but live microphone capture requires a secure context. `localhost` is trusted on the same device; a phone visiting a computer's LAN address needs a trusted HTTPS frontend. Serve the API over HTTPS too (for example through a TLS reverse proxy), set `NEXT_PUBLIC_API_URL` to that HTTPS API address, set `FRONTEND_ORIGIN` to the exact HTTPS frontend origin, and configure `BACKEND_HOST` so the proxy can reach the API. The phone must trust both certificates. Exposing the single-user API also exposes its saved collection and audio to anyone who can reach it.

## Configuration

The backend reads these optional environment variables:

| Variable | Default | Purpose |
| --- | --- | --- |
| `BACKEND_HOST` / `BACKEND_PORT` | `127.0.0.1` / `8080` | HTTP listener |
| `DATABASE_PATH` | `../data/ecosound.sqlite` | Persistent SQLite database |
| `AUDIO_STORAGE_DIR` | `../data/audio` | Private normalized WAV files |
| `PYTHON_EXECUTABLE` | `../.venv/bin/python` | BirdNET environment interpreter |
| `INFERENCE_SCRIPT` | `../inference/identify.py` | Python adapter path |
| `INFERENCE_TIMEOUT` | `180s` | Maximum model call time, capped at `210s` |
| `INFERENCE_PROVIDER` | `local` | Use `local` BirdNET or authenticated `modal` HTTP inference |
| `BIRDNET_MODAL_URL` | empty | Deployed BirdNET Modal service URL; backend only |
| `MODAL_PROXY_TOKEN_ID` / `MODAL_PROXY_TOKEN_SECRET` | empty | BirdNET endpoint proxy credentials; backend only |
| `QUEST_TIMEZONE` | `Africa/Nairobi` | Calendar day used by the quest |
| `FRONTEND_ORIGIN` | `http://localhost:3000` | Browser origin allowed by CORS |
| `BIRDNET_APP_DATA` | BirdNET package default | Model asset cache; `.env.example` points to `../data/birdnet` |
| `QWEN_ENABLED` / `QWEN_PROVIDER` | `false` / `local` | Enable the guide/adventure model and choose local or Modal |
| `QWEN_MODEL_PATH` / `QWEN_ENDPOINT` | `qwen3:1.7b` / local Ollama URL | Local OpenAI-compatible Qwen service settings |
| `QWEN_MODAL_URL` | empty | Deployed Qwen Modal base URL; backend only |
| `QWEN_MODAL_PROXY_TOKEN_ID` / `QWEN_MODAL_PROXY_TOKEN_SECRET` | empty | Qwen endpoint proxy credentials; backend only |
| `QWEN_TIMEOUT_SECONDS` | `300` | Per Qwen model check or generation timeout, capped at `300s` |
| `NATURELM_ENABLED` / `NATURELM_MODEL_PATH` / `NATURELM_ENDPOINT` | `false` / official model ID / loopback sidecar | Optional NatureLM audio descriptions; never a verified species label |
| `NATURELM_TIMEOUT_SECONDS` | `120` | NatureLM call timeout |
| `PUBLIC_MAP_ENABLED` | `true` | Enables the generalized public observations endpoint |
| `OBSERVATION_LOCATION_PRECISION` | `coarse` | `coarse` rounds public map coordinates to 2 decimal places; `very_coarse` to 1 |
| `NEXT_PUBLIC_OFFLINE_CACHE_ENABLED` | `true` | Enables the browser's best-effort IndexedDB JSON cache |

Paths above are relative to the backend working directory. The frontend reads `NEXT_PUBLIC_API_URL`, defaulting to `http://localhost:8080`.

## Check it

From `ecosound`:

```sh
make test
# Or run the two backend checks directly:
python3 -m unittest discover -s inference -p 'test_*.py' -v
cd backend && go test ./...
```

`make test` also runs the frontend test suite. `make setup` installs Ruff from `inference/requirements-dev.txt` alongside the Python runtime dependency. If using the direct runtime setup above, install that development requirements file before linting. `make lint` checks Go formatting/vet, Python syntax and Ruff, and frontend lint/typecheck; `make build` builds the server and frontend. `make verify-inference AUDIO=/absolute/path/decoded-mono.wav` runs the real model adapter directly and requires its assets to be available.

For a demo, prepare a real 1–60 second bird clip as mono 32 kHz, 16-bit PCM WAV, then warm the model before presenting:

```sh
ffmpeg -i input-audio.ext -vn -ac 1 -ar 32000 -c:a pcm_s16le -t 60 demo-bird.wav
make verify-inference AUDIO=/absolute/path/demo-bird.wav
```

Check that the converted clip is at least one second long. The first verification can download model assets and take longer than later calls. Modal BirdNET was verified through the Go API with real licensed Blackbird and Hadada recordings; see [Modal deployment and verification](docs/modal-deployment.md). Use a recording you are permitted to demo and treat returned species as candidates.

With the API running, `scripts/smoke.sh` checks health. Pass a real audio file to invoke BirdNET, save its first candidate if one exists, and print quest progress:

```sh
./scripts/smoke.sh path/to/bird-recording.wav
```

Saving via the smoke script changes the local collection. A no-detection result saves nothing. See [validation.md](docs/validation.md) for manual checks and the current model verification status.

## Data, accuracy, and licensing

Each upload becomes a recording with server receipt time. BirdNET returns zero or more timestamped candidates. Saving one candidate creates a discovery; a second candidate from the same recording cannot be saved. Confirmation records only the user's own judgment and displays **“Personally confirmed — not independently scientifically verified.”** The quest counts saved discoveries, including repeated calls of one species for its calls task, while its species task counts distinct plausible scientific names. The data stays in the local SQLite database and audio directory; EcoSound does not retrain BirdNET or publish recordings.

The application code is MIT licensed. The separate BirdNET model has [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/) terms, including a noncommercial restriction. See [third-party notices](docs/THIRD_PARTY_NOTICES.md) before packaging or broader use.

## Outdoor adventures and community features

The **Explore** view creates and persists short outdoor missions. Qwen can run on a local OpenAI-compatible service or the independently deployed L4 Modal service. Its Nature Guide explanation is grounded in the curated species record, includes three general outdoor activities, and keeps the BirdNET result labeled as a prediction. If unavailable or malformed, the app shows curated facts without inventing model output. See [AI adventures](docs/ai-adventures.md) and [Modal deployment](docs/modal-deployment.md).

BirdNET results open in the Nature Guide. Facts are retrieved from a small local curated store before optional Qwen summaries, with source attribution and a no-data response. Community observations require explicit consent; locations are generalized for the public map and unmapped/sensitive species remain hidden. A route-free Karura Forest concept is marked unaffiliated and has no fabricated trail. See [education](docs/wildlife-education.md), [map](docs/biodiversity-map.md), and [destinations](docs/destination-integration.md).

The application is a PWA. Previously loaded JSON and adventure instructions are cached in the browser; recordings and queued actions can wait for connectivity and synchronize idempotently. BirdNET, Qwen generation/explanations, NatureLM, new server data, and map tiles need a connection. This is not on-device inference. The optional NatureLM audio description adapter is disabled by default; gated model access and real inference were unavailable in this environment. See [offline architecture](docs/offline-architecture.md), [NatureLM](docs/naturelm-integration.md), and [privacy](docs/privacy.md).

Model licenses are distinct: Qwen3-1.7B is Apache-2.0, while BirdNET weights remain CC BY-NC-SA 4.0. NatureLM deployment additionally depends on its upstream terms and access to the separately licensed base model. No weights or recordings are committed. MapLibre GL JS is MIT; hosted tiles follow OpenFreeMap terms and OpenStreetMap attribution. Review [third-party notices](docs/THIRD_PARTY_NOTICES.md).

For implementation details, see the [architecture](docs/architecture.md) and [API contract](docs/api-contract.md). The [hackathon write-up](docs/hackathon-submission.md) includes the problem statement, limitations, future work, and demo script.
