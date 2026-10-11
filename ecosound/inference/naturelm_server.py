#!/usr/bin/env python3
"""Persistent local HTTP adapter; loads official NatureLM weights only once."""
from __future__ import annotations

import json
import os
import sys
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

from naturelm_adapter import normalize_outputs, validate_wav

MAX_BYTES = 15 * 1024 * 1024
PIPELINE = None
INFERENCE_LOCK = threading.Lock()


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        print("NatureLM server: " + fmt % args, file=sys.stderr)

    def send_json(self, status: int, value: dict) -> None:
        data = json.dumps(value, allow_nan=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path != "/health":
            self.send_json(404, {"error": "not found"})
            return
        self.send_json(200, {"status": "ready", "provider": "naturelm-audio"})

    def do_POST(self):
        if self.path != "/infer":
            self.send_json(404, {"error": "not found"})
            return
        length = self.headers.get("Content-Length", "")
        if not length.isdigit() or int(length) < 1 or int(length) > MAX_BYTES:
            self.send_json(413, {"error": "audio payload exceeds 15 MiB"})
            return
        task = self.headers.get("X-EcoSound-Task", "audio_description")
        if task not in {"audio_description", "vocalization_characteristics"}:
            self.send_json(400, {"error": "unsupported task"})
            return
        try:
            data = self.rfile.read(int(length))
            if len(data) != int(length):
                raise ValueError("truncated request")
            with tempfile.TemporaryDirectory(prefix="ecosound-naturelm-") as tmp:
                audio = Path(tmp) / "recording.wav"
                audio.write_bytes(data)
                validate_wav(audio)
                query = (
                    "Describe the audible animal sounds and vocalization characteristics. "
                    "Do not claim a species identification; describe uncertainty."
                    if task == "audio_description"
                    else "Describe the audible vocalization's rhythm, pitch, and texture. Do not identify a species."
                )
                # Official Pipeline API takes a list of audio paths and a text query.
                with INFERENCE_LOCK:
                    outputs = PIPELINE([str(audio)], [query], window_length_seconds=10.0, hop_length_seconds=10.0)
                self.send_json(200, normalize_outputs(outputs, task))
        except Exception as exc:
            print(f"NatureLM inference failed: {type(exc).__name__}: {exc}", file=sys.stderr)
            self.send_json(422, {"error": "NatureLM could not process this audio"})


def main() -> None:
    repo = os.environ.get("NATURELM_REPO_PATH", "")
    if repo:
        sys.path.insert(0, repo)
    model_name = os.environ.get("NATURELM_MODEL_PATH", "EarthSpeciesProject/NatureLM-audio")
    import torch
    from NatureLM.infer import Pipeline
    from NatureLM.models import NatureLM

    device = "cuda" if torch.cuda.is_available() else "cpu"
    print(f"Loading official NatureLM-audio from {model_name} on {device}; model access and suitable memory are required.", file=sys.stderr)
    model = NatureLM.from_pretrained(model_name).eval().to(device)
    global PIPELINE
    PIPELINE = Pipeline(model=model)
    host = os.environ.get("NATURELM_HOST", "127.0.0.1")
    port = int(os.environ.get("NATURELM_PORT", "8091"))
    ThreadingHTTPServer((host, port), Handler).serve_forever()


if __name__ == "__main__":
    main()
