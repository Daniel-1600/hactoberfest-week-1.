#!/usr/bin/env python3
"""Real BirdNET 3.0 ONNX inference. Stdout is exclusively the JSON contract."""

import argparse
import contextlib
import json
import math
import os
import sys
import wave
from pathlib import Path


def normalize_predictions(result):
    """Adapt BirdNET AcousticPredictionResultBase.to_structured_array(), not JSON.

    Upstream fields: input, start_time, end_time, species_name, confidence.
    Masked detections are already removed by upstream. Keep every returned row.
    """
    if len(result.unprocessable_inputs):
        raise ValueError("BirdNET could not process the audio")
    rows = result.to_structured_array()
    expected = {"species_name", "confidence", "start_time", "end_time"}
    if not expected.issubset(set(rows.dtype.names or ())):
        raise ValueError("Unsupported BirdNET result schema")
    predictions = []
    for row in rows:
        raw = str(row["species_name"])
        scientific, separator, common = raw.partition("_")
        score = float(row["confidence"])
        start, end = float(row["start_time"]), float(row["end_time"])
        if not all(math.isfinite(v) for v in (score, start, end)):
            raise ValueError("Non-finite BirdNET output")
        if not 0 <= score <= 1 or not 0 <= start <= end:
            raise ValueError("Invalid BirdNET score or timestamps")
        predictions.append(
            {
                "raw_species_name": raw,
                "scientific_name": scientific,
                "common_name": common if separator else "",
                "confidence": score,
                "start_time": start,
                "end_time": end,
            }
        )
    return {"model": "birdnet", "model_version": "3.0", "predictions": predictions}


@contextlib.contextmanager
def diagnostics_to_stderr():
    # Redirect fd 1 as well: native libraries and spawned BirdNET workers inherit it.
    sys.stdout.flush()
    saved = os.dup(1)
    try:
        os.dup2(2, 1)
        with contextlib.redirect_stdout(sys.stderr):
            yield
    finally:
        os.dup2(saved, 1)
        os.close(saved)


def validate_audio(path):
    # The Go boundary converts every accepted format to uncompressed mono PCM WAV.
    if not path.is_file():
        raise ValueError("Audio file does not exist")
    with wave.open(str(path), "rb") as audio:
        duration = audio.getnframes() / audio.getframerate()
        if audio.getcomptype() != "NONE" or audio.getnchannels() != 1:
            raise ValueError("Expected decoded mono PCM WAV")
        if not 1 <= duration <= 60.1:
            raise ValueError("Audio must be between 1 and 60 seconds")
        # Read the whole bounded recording to reject truncated PCM payloads.
        pcm = audio.readframes(audio.getnframes())
        if len(pcm) != audio.getnframes() * audio.getsampwidth():
            raise ValueError("Truncated audio")


def identify(path, threshold=0.25):
    validate_audio(path)
    with diagnostics_to_stderr():
        import birdnet

        print(
            "Loading BirdNET acoustic 3.0 ONNX (first run downloads model assets)",
            file=sys.stderr,
        )
        model = birdnet.load("acoustic", "3.0", "onnx")
        result = model.predict(
            str(path),
            n_workers=1,
            n_producers=1,
            top_k=5,
            default_confidence_threshold=threshold,
            show_stats=None,
        )
        return normalize_predictions(result)


def main(argv=None):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("audio", type=Path)
    parser.add_argument("--threshold", type=float, default=0.25)
    args = parser.parse_args(argv)
    try:
        if not 0 <= args.threshold <= 1:
            raise ValueError("Threshold must be between zero and one")
        output = identify(args.audio.resolve(), args.threshold)
        print(json.dumps(output, allow_nan=False, ensure_ascii=False))
        return 0
    except Exception as exc:
        print(f"BirdNET inference failed: {type(exc).__name__}: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
