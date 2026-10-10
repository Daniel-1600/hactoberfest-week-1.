# EcoSound architecture

EcoSound is a local, single-user audio discovery app. The browser records or uploads a short sound, the Go server validates and stores it, and BirdNET 3.0 returns candidate bird labels. A person can save one candidate per recording to a private collection. The daily quest derives its progress from those saved records.

```text
Browser (React)
  │ multipart audio / JSON
  ▼
Go HTTP API ─── SQLite: recordings, predictions, discoveries
  │  │
  │  └── private normalized WAV storage
  │
  ├── FFprobe: inspect actual media and duration
  ├── FFmpeg: decode to mono PCM WAV
  └── Python subprocess: inference/identify.py
         └── BirdNET 3.0 ONNX model
```

An observed event is the physical listening encounter outdoors. The MVP has no separate event table, location proof, or independent capture timestamp. A `recording` represents audio submitted to the server and its receipt time is the proxy observation time, including for a file uploaded later. `predictions` represent model candidates at time windows; `discoveries` represent the user's choice to save one prediction from a recording. The server owns IDs and timestamps. Saving does not change the model output. A later confirmation means the user personally confirmed that choice; it is not independent scientific verification.

The HTTP contract is in [api-contract.md](api-contract.md). Uploads are limited to 15 MiB and decoded audio to 1–60 seconds. A file extension alone is insufficient: FFprobe checks content and FFmpeg normalizes accepted formats. The Python process emits one JSON object to stdout and diagnostics to stderr. The Go boundary treats process failure, timeout, and malformed output as errors. An empty valid prediction list is a successful no-detection result and earns no discovery.

Quest progress uses server receipt time as the observation time, converted into the configured local timezone for a calendar day. “Record three distinct calls” counts distinct saved recording IDs. “Discover two distinct species” counts distinct plausible binomial or trinomial scientific names among those discoveries; noise and unknown labels do not earn species credit. Repeated saves from one recording are prohibited; repeated instances of one species can advance the calls task but not the species task. Confirmation has no effect on either count.

The data and audio are local to the running instance. There are no accounts or access controls, so anyone who can reach the API can use this one collection and request stored audio. Bind it to localhost unless that sharing is intended. EcoSound does not publish recordings or use them to train BirdNET.

The model is a candidate generator. Scores are model confidence values, not calibrated guarantees of correct identification. Environmental noise, overlap, geography, and species outside the model's classes can produce missing or wrong suggestions. Human review and, where relevant, expert validation remain necessary before treating a species record as evidence.

## Future evaluation opportunities

An opt-in evaluation workflow could collect recording consent and licensing, preserve original model outputs, and obtain independent expert labels. A curated held-out set could then compare model mistakes, species coverage, and performance across habitats and recording conditions. Personal confirmations alone are not ground truth for that evaluation. EcoSound has neither created a research dataset nor improved, fine-tuned, or retrained BirdNET.

## Runtime boundaries

Only one inference job runs at a time; concurrent identification requests receive a retryable busy response. Python has a configurable timeout. On Linux, macOS, and FreeBSD, cancellation kills the whole subprocess group, including BirdNET's workers. The generic fallback on other systems cancels the parent process only; use a Unix host for the documented MVP workflow. SQLite foreign keys and migrations apply at startup, with a single connection for consistent enforcement. Audio and database writes are not a cross-filesystem transaction: handled failures remove audio, but an abrupt machine shutdown during a write may leave an unreferenced WAV requiring local cleanup.
