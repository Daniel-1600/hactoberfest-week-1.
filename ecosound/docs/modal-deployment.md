# Independent Modal model services

EcoSound can keep BirdNET and Qwen on separate Modal apps. Each app has its own endpoint, cache volume, scaling, and proxy token. Both web services scale to zero after one minute idle; requests incur compute only while Modal is serving or warming a model. BirdNET uses CPU. Qwen requests one L4 GPU. The BirdNET model is CC BY-NC-SA 4.0; review its noncommercial terms before deployment beyond permitted use.

## BirdNET CPU service

Run these commands from `ecosound/`:

```sh
modal deploy modal/birdnet_app.py
modal run --env main modal/birdnet_app.py::warm_model
```

The deployment prints a proxy-authenticated URL. The warmup downloads acoustic 3.0 ONNX weights and taxonomy to the `ecosound-birdnet-model-cache` Modal Volume and commits them. The warmup function has a longer timeout for the initial download; routine endpoint requests use the regular inference timeout. It accepts `audio/wav` bytes containing the backend's normalized mono PCM WAV, validates the 1–60 second duration, and calls the existing `inference/identify.py`. Successful JSON responses retain the established `model`, `model_version`, and `predictions` contract. Errors do not produce a success-shaped or fabricated result.

Create a Modal Proxy Token in the same Modal environment as the deployment. Proxy tokens are distinct from Modal API tokens. Keep the token ID and secret in the Go backend process environment only:

```sh
INFERENCE_PROVIDER=modal
BIRDNET_MODAL_URL=https://YOUR-WORKSPACE--ecosound-birdnet-web-app.modal.run
MODAL_PROXY_TOKEN_ID=wk-...
MODAL_PROXY_TOKEN_SECRET=ws-...
INFERENCE_TIMEOUT=180s
```

Do not put these values in `frontend/.env*` or `NEXT_PUBLIC_*` variables. To return to local inference, set `INFERENCE_PROVIDER=local`; local inference continues to use `PYTHON_EXECUTABLE` and `INFERENCE_SCRIPT`.

Verify the deployed endpoint through the Go API with a real recording that you have permission to use:

```sh
ffmpeg -i input-audio.ext -vn -ac 1 -ar 32000 -c:a pcm_s16le -t 60 normalized.wav
curl -F 'audio=@normalized.wav;type=audio/wav' http://localhost:8080/api/identify
```

The endpoint returns the normal EcoSound identification response with actual predictions from BirdNET. A zero-prediction result is valid; the service never adds sample detections. The first model fetch can take several minutes, so run the cache warmup before a live demo.

## Qwen L4 service

The Qwen deployment is independent:

```sh
modal deploy modal/qwen_app.py
modal run --env main modal/qwen_app.py::cache_model
```

`cache_model` stores `Qwen/Qwen3-1.7B` in the `ecosound-qwen-huggingface-cache` volume without allocating a GPU. vLLM serves the OpenAI-compatible API on an L4, with vLLM artifacts in a separate persistent volume. The authenticated web server scales to zero after one minute idle. The deployment does not start or reserve an L4 until a request needs a container.

Enable it on the Go backend by setting:

```sh
QWEN_ENABLED=true
QWEN_PROVIDER=modal
QWEN_MODAL_URL=https://YOUR-WORKSPACE--ecosound-qwen-nature-guide-serve-qwen.modal.run
QWEN_MODEL_PATH=qwen3-1.7b
QWEN_MODAL_PROXY_TOKEN_ID=wk-...
QWEN_MODAL_PROXY_TOKEN_SECRET=ws-...
QWEN_TIMEOUT_SECONDS=60
```

For local Ollama or another compatible server, use `QWEN_PROVIDER=local`, `QWEN_ENDPOINT`, and the local model name. Keep `QWEN_ENABLED=false` to turn off model-generated explanations and activities while retaining curated species facts. Qwen is also used by the existing adventure generator when enabled.

The species explanation API works from a saved EcoSound discovery. The Go service passes the database prediction score and curated species record to Qwen, requests a short explanation and three general low-impact outdoor activities, and labels the identification as an unverified BirdNET suggestion. It rejects malformed output, unsupported response fields, overlong activities, and direct claims that the identification is confirmed. The activities do not assert that the bird is present. A model failure produces an API error; the interface falls back to the cited curated facts and does not fabricate an AI response.

## Verification

Run the local contract tests:

```sh
python3 -m unittest discover -s modal -p 'test_*.py' -v
cd backend && go test ./...
cd ../frontend && npm test -- --run
```

For an end-to-end BirdNET check, verify the HTTP response reports `birdnet` / `3.0` and inspect the returned real candidates. For Qwen, save one returned BirdNET prediction as a discovery, call `POST /api/discoveries/{id}/explain`, and check that the response includes cited curated facts, three activities, and the unverified-prediction disclaimer. An authenticated request without a valid Modal Proxy Token should receive 401 before it reaches either model service.

## References

- [Modal Web Functions and proxy authentication](https://modal.com/docs/guide/webhook-proxy-auth)
- [Modal Volumes](https://modal.com/docs/guide/volumes)
- [Modal vLLM OpenAI-compatible serving example](https://modal.com/docs/examples/llm_inference)
- [Qwen3-1.7B model card and license](https://huggingface.co/Qwen/Qwen3-1.7B)
- [BirdNET package and model backends](https://pypi.org/project/birdnet/)
