"""Modal CPU deployment for the existing BirdNET 3.0 ONNX adapter.

Deploy from the ecosound directory with `modal deploy modal/birdnet_app.py`.
"""

from pathlib import Path
import sys

import modal

from birdnet_service import infer_wav

ROOT = Path(__file__).resolve().parents[1]
APP_NAME = "ecosound-birdnet"
MODEL_DIR = "/model-cache/birdnet"

volume = modal.Volume.from_name("ecosound-birdnet-model-cache", create_if_missing=True)
image = (
    modal.Image.debian_slim(python_version="3.11")
    .apt_install("libgomp1")
    .pip_install("birdnet==1.1.1", "fastapi")
    .env({"BIRDNET_APP_DATA": MODEL_DIR, "PYTHONPATH": "/root"})
    .add_local_file(str(ROOT / "inference" / "identify.py"), "/root/identify.py")
    .add_local_file(str(Path(__file__).with_name("birdnet_service.py")), "/root/birdnet_service.py")
)
app = modal.App(APP_NAME, image=image)


@app.function(
    cpu=4,
    memory=8192,
    timeout=600,
    scaledown_window=60,
    volumes={MODEL_DIR: volume},
)
def warm_model():
    """Download and persist ONNX assets separately from latency-bound requests."""
    sys.path.insert(0, "/root")
    import birdnet

    birdnet.load("acoustic", "3.0", "onnx")
    volume.commit()


@app.function(
    cpu=4,
    memory=8192,
    timeout=210,
    max_containers=1,
    scaledown_window=60,
    volumes={MODEL_DIR: volume},
)
@modal.asgi_app(requires_proxy_auth=True)
def web_app():
    from fastapi import FastAPI, HTTPException, Request
    from fastapi.responses import JSONResponse

    service = FastAPI(docs_url=None, redoc_url=None, openapi_url=None)

    @service.post("/")
    async def identify_endpoint(request: Request):
        if request.headers.get("content-type", "").split(";", 1)[0].strip().lower() != "audio/wav":
            raise HTTPException(status_code=415, detail="Content-Type must be audio/wav")
        payload = bytearray()
        async for chunk in request.stream():
            payload.extend(chunk)
            if len(payload) > 8 * 1024 * 1024:
                raise HTTPException(status_code=413, detail="WAV exceeds 8 MiB")
        payload = bytes(payload)
        try:
            sys.path.insert(0, "/root")
            from identify import identify

            result = infer_wav(payload, identify)
            volume.commit()
            return JSONResponse(result)
        except ValueError as exc:
            raise HTTPException(status_code=422, detail=str(exc)) from exc
        except Exception:
            # Avoid returning model paths, cache details, or runtime diagnostics to callers.
            raise HTTPException(status_code=503, detail="BirdNET inference unavailable")

    return service
