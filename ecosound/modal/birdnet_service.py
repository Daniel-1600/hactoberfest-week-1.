"""Small request adapter shared by the Modal app and its offline tests."""

import io
import json
import tempfile
import wave
from pathlib import Path
from typing import Callable

MAX_AUDIO_BYTES = 8 * 1024 * 1024


def infer_wav(payload: bytes, identify: Callable[[Path], dict]) -> dict:
    """Validate normalized WAV bytes and return identify.py's contract unchanged."""
    if not payload or len(payload) > MAX_AUDIO_BYTES:
        raise ValueError("Audio must be non-empty and no larger than 8 MiB")
    try:
        with wave.open(io.BytesIO(payload), "rb") as audio:
            duration = audio.getnframes() / audio.getframerate()
            if audio.getcomptype() != "NONE" or audio.getnchannels() != 1:
                raise ValueError("Expected decoded mono PCM WAV")
            if not 1 <= duration <= 60.1:
                raise ValueError("Audio must be between 1 and 60 seconds")
            frames = audio.readframes(audio.getnframes())
            if len(frames) != audio.getnframes() * audio.getsampwidth():
                raise ValueError("Truncated audio")
    except (wave.Error, EOFError) as exc:
        raise ValueError("Invalid WAV audio") from exc

    with tempfile.TemporaryDirectory(prefix="ecosound-birdnet-") as directory:
        path = Path(directory) / "normalized.wav"
        path.write_bytes(payload)
        result = identify(path)
    # JSON round-trip keeps the service boundary JSON-safe without changing fields.
    return json.loads(json.dumps(result, allow_nan=False))
