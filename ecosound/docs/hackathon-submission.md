# EcoSound — Turn Outdoor Exploration Into Wildlife Discovery

## Problem

People spend more time indoors and on screens, while everyday wildlife sounds pass unnoticed. A bird call on a short walk is easy to forget and hard for a newcomer to identify. Most people have no simple way to connect that listening moment to a record they can replay and review.

## Solution

EcoSound turns outdoor listening into short missions supported by open-weight AI. A local Qwen service can generate a personalized adventure from the user's time, interests, environment, difficulty, and optional approved destination guidance. Users record or upload a bird sound, review BirdNET candidates, save a private discovery, learn from curated source-backed species information, and optionally contribute a privacy-generalized observation. The original daily quest remains independent. Adventures encourage an attentive walk without presenting an AI score as proof of a species.

## How it works

1. Generate an adventure using Qwen through a persistent local OpenAI-compatible model endpoint. The Go service validates structured output, rejects unsafe tasks, and clearly labels a safe fallback if generation is unavailable.
2. Start the adventure and put the phone away; later capture or upload 1–60 seconds of audio. The server validates the content and uses FFmpeg to create a mono WAV.
3. BirdNET 3.0 analyzes the clip locally and returns candidate labels, scores, and time windows. No detection is a valid outcome. NatureLM-audio is an optional experimental description provider, not a species classifier.
4. Save one candidate per recording in a private collection. Curated species records ground short educational explanations. Replay the audio and, if personally reviewed, mark the choice “Personally confirmed — not independently scientifically verified.”
5. Optionally provide explicit consent to contribute an observation. The public map receives generalized coordinates only. Adventure progress and XP are persisted; sync operations are queued while offline. BirdNET, Qwen, NatureLM, and public observation submission still require a reachable server/model service.
6. Morning Bird Quest derives its counts from saved recordings and distinct plausible scientific names in the configured local day. Counts come from stored discoveries, not seeded demo progress.

The interface uses React and Next.js; the API uses Go, SQLite, and private local WAV storage. FFprobe checks uploaded media, FFmpeg decodes it, and a Python subprocess runs BirdNET. An observed event means the real listening encounter; the MVP uses server receipt time as its proxy observation time, including for later file uploads. Destinations and experiences are data-driven; the route-free Karura-inspired content is explicitly a concept demonstration and is not affiliated with forest management.

## Open-source AI

EcoSound calls the official BirdNET 3.0 ONNX model through the open-source `birdnet` Python package. Qwen3-1.7B powers optional adventure generation and grounded explanations via a local model service. NatureLM-audio is optional and reports audio descriptions only. These models run on locally deployable services, so the application does not depend on paid inference APIs. BirdNET's original label, score, and time window remain available for review. Model weights and gated access terms have separate licensing from the MIT-licensed app code; see [third-party notices](THIRD_PARTY_NOTICES.md).

## Why open

An open implementation makes the capture, model boundary, scoring display, and quest rules transparent and reproducible. Communities can adapt the local app to their own biodiversity context, inspect how a saved discovery was formed, and choose whether any future dataset sharing is appropriate. The current private recordings are not a research dataset.

## Limitations

- Predictions are not guaranteed correct, especially with noise, overlapping calls, or unfamiliar species.
- Human confirmation is personal, not independent scientific verification.
- The MVP focuses on bird identification rather than identifying every type of animal.
- Model confidence is an AI identification score and should not be interpreted as guaranteed accuracy.
- Uploaded recordings are not automatically research-grade datasets or published open datasets.

BirdNET installation and the first model asset download were blocked by sandbox DNS during preparation. The MVP has one local collection without accounts or authentication. Server receipt time is a proxy for observation time; there is no location or capture-time verification. Each recording can contribute at most one saved discovery.

## Future Improvements

1. Integrate Earth Species Project NatureLM-audio for broader sound interpretation.
2. Compare multiple models and show where their candidate labels disagree.
3. Add expert-reviewed annotations and a clear verification trail.
4. Offer an explicitly opt-in research dataset export with consent and provenance checks.
5. Run inference offline on mobile devices.
6. Expand beyond birds to frogs, insects, and other animal groups.

Expert review, opt-in research export, on-device inference, multi-account identity, and additional animal classifiers remain future work. The Qwen, BirdNET, and NatureLM model paths are configurable optional external local services; this environment did not have their weights/runtime available for real model verification. See each feature note and [validation.md](validation.md) for precise status.

## 90-second demo script

Before filming, install BirdNET and download its model assets. Save two **real, distinct recordings** of one recognized species through the app earlier on the same Nairobi day, leaving calls at 2/3 and species at 1/2. Prepare another genuine recording whose real result identifies a second species. These are prior model-backed discoveries, never database seeds or fixture predictions. Without this preparation, show the actual incomplete counts. Inference latency must be measured on the demo machine; a 90-second run has not been verified in the restricted build environment.

**0–15 seconds:** Open Explore, choose duration, interest, and environment, and generate an adventure. Show whether it came from Qwen or the clearly labeled fallback. Start it and point out the phone-away prompt.

**15–40 seconds:** Use a previously prepared real recording for a quick run, or capture a clear call. Play it back, identify it, and show actual BirdNET output. Mention that no detection is possible and the score is not certainty. If inference is unverified on the demo machine, stop short of implying real results.

**40–60 seconds:** Save a candidate and open Learn About This Species. Point out the curated source and short grounded explanation; if the species is not in the small knowledge store, show the unavailable-data message.

**60–80 seconds:** Contribute an observation only after explicit consent, then show its generalized map marker and verification label. Mention that precise coordinates and recordings remain private.

**80–90 seconds:** Finish the adventure and show the XP/passport update and self-reported reflection. Clarify that cached adventures and queued recordings work offline, while AI inference and public map sync require connectivity.

## Screenshot and demo guidance

Capture the recording/upload state, a real inference result with visible candidate details, the collection entry with playback, and the quest showing its actual counts. Prepare a real mono 32 kHz, 16-bit PCM WAV clip with `ffmpeg -i input-audio.ext -vn -ac 1 -ar 32000 -c:a pcm_s16le -t 60 demo-bird.wav`, and run `make verify-inference AUDIO=/absolute/path/demo-bird.wav` before the 90-second demo so the model can download and warm up. Ensure the clip is at least one second long. No official sample download was verified in the restricted build environment. If model assets cannot be downloaded in the demo environment, show the interface and validation behavior but label inference unverified; do not present fixture candidates as real identifications. Avoid screenshots that expose local paths, private recordings, or personal data without consent.

## Validation disclosure

At initial preparation, Python adapter tests and the Go test suite passed, but sandbox DNS failure prevented installing BirdNET and downloading model assets. The sandbox also denied local socket access for a running-server browser walkthrough. No real species identification was observed. See [validation.md](validation.md) for checks and the manual end-to-end procedure.
