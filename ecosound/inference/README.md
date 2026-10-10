# BirdNET inference boundary

`identify.py` is the real inference adapter used by the Go server. It accepts a path to a decoded mono PCM WAV file of 1–60 seconds, loads BirdNET acoustic 3.0 through the official `birdnet==1.1.1` Python package, and writes exactly one JSON result to stdout. Diagnostics go to stderr. It does not contain a mock mode or sample detections.

Install in a Python virtual environment from the `ecosound` directory:

```sh
python3 -m venv .venv
. .venv/bin/activate
python -m pip install -r inference/requirements.txt
python inference/identify.py path/to/decoded-mono.wav
```

The first invocation downloads model assets and therefore needs network access. Set the backend's Python executable to the virtual environment's interpreter (see the root README). The Go server performs FFprobe/FFmpeg normalization before calling this script; direct CLI use requires an already decoded mono WAV.

The output has `model`, `model_version`, and `predictions`. Each prediction retains BirdNET's original label, its split scientific/common name, unrounded score, and start/end seconds. The adapter checks `unprocessable_inputs` before accepting an empty result. A valid no-detection result has an empty `predictions` array. A model/process error exits 1, while CLI argument errors exit 2; neither prints a JSON success object.

Run adapter tests without model assets:

```sh
python3 -m unittest discover -s inference -p 'test_*.py' -v
```

These tests validate normalization and the local CLI/audio boundary. They cannot establish that BirdNET installs, downloads, or correctly identifies a bird on this machine. See [validation.md](../docs/validation.md) for the current end-to-end status.
