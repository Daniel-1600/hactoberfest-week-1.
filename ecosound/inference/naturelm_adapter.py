"""Small, testable adapter for documented NatureLM Pipeline text outputs."""
from __future__ import annotations

import re
import wave
from pathlib import Path

SEGMENT = re.compile(r"^#([0-9]+(?:\.[0-9]+)?)s\s*-\s*([0-9]+(?:\.[0-9]+)?)s#:\s*(.*)$")


def validate_wav(path: Path) -> None:
    with wave.open(str(path), "rb") as audio:
        duration = audio.getnframes() / audio.getframerate()
        if audio.getcomptype() != "NONE" or audio.getnchannels() != 1 or not 1 <= duration <= 60.1:
            raise ValueError("Expected a 1–60 second mono PCM WAV")
        payload = audio.readframes(audio.getnframes())
        if len(payload) != audio.getnframes() * audio.getsampwidth():
            raise ValueError("Truncated WAV")


def normalize_outputs(outputs: object, task: str) -> dict:
    if task not in {"audio_description", "vocalization_characteristics"}:
        raise ValueError("Unsupported NatureLM task")
    if not isinstance(outputs, (list, tuple)) or not 1 <= len(outputs) <= 30:
        raise ValueError("NatureLM returned an unsupported output shape")
    normalized = []
    for output in outputs:
        if not isinstance(output, str):
            raise ValueError("NatureLM output must be text")
        text = output.strip()
        if not text or len(text) > 2000:
            raise ValueError("NatureLM output is empty or too long")
        normalized.append(text)
    return {
        "provider": "naturelm-audio",
        "task": task,
        "outputs": normalized,
        "species_identification": False,
    }
