# EcoSound — Turn Outdoor Exploration Into Wildlife Discovery

## Problem

People spend more time indoors and on screens, while everyday wildlife sounds pass unnoticed. A bird call on a short walk is easy to forget and hard for a newcomer to identify. Most people have no simple way to connect that listening moment to a record they can replay and review.

## Solution

EcoSound turns outdoor listening into short missions supported by open-source bioacoustics AI. Record or upload a bird sound, review BirdNET candidates, save one personal discovery with its audio, and follow a daily quest to find three distinct calls and two distinct species. The loop encourages another attentive walk without presenting an AI score as proof of a species.

## How it works

1. Capture or upload 1–60 seconds of audio. The server validates the content and uses FFmpeg to create a mono WAV.
2. BirdNET 3.0 analyzes the clip locally and returns candidate labels, scores, and time windows. No detection is a valid outcome.
3. Save one candidate per recording in a private collection. Replay the audio and, if personally reviewed, mark the choice “Personally confirmed — not independently scientifically verified.”
4. Morning Bird Quest derives its counts from saved recordings and distinct plausible scientific names in the configured local day. Counts come from stored discoveries, not seeded demo progress.

The interface uses React and Next.js; the API uses Go, SQLite, and private local WAV storage. FFprobe checks uploaded media, FFmpeg decodes it, and a Python subprocess runs BirdNET. An observed event means the real listening encounter; the MVP uses server receipt time as its proxy observation time, including for later file uploads.

## Open-source AI

EcoSound calls the official BirdNET 3.0 ONNX model through the open-source `birdnet` Python package. The model runs on a locally deployable server, so the application does not need to send recordings to a hosted identification API. It preserves BirdNET's original label, score, and time window for review. The model's weights have separate CC BY-NC-SA 4.0 terms; the app code is MIT licensed.

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

These are proposals, not implemented features.

## 90-second demo script

Before filming, install BirdNET and download its model assets. Save two **real, distinct recordings** of one recognized species through the app earlier on the same Nairobi day, leaving calls at 2/3 and species at 1/2. Prepare another genuine recording whose real result identifies a second species. These are prior model-backed discoveries, never database seeds or fixture predictions. Without this preparation, show the actual incomplete counts. Inference latency must be measured on the demo machine; a 90-second run has not been verified in the restricted build environment.

**0–15 seconds:** Open EcoSound, show Morning Bird Quest and its honest 2/3 and 1/2 progress. Explain that the next outdoor listening moment can finish today's adventure. Select Start quest.

**15–40 seconds:** Record a clear call for a few seconds and stop, or upload the prepared authentic bird recording. Play it back, select Identify this sound, and show actual BirdNET processing and results. Mention that no detection is possible and that a model score is not certainty.

**40–60 seconds:** Save the second-species candidate. Show the automatically refreshed “Quest complete! You explored the natural world today.” message. Explain that the database counted three distinct recordings and two species, without manual checkboxes.

**60–80 seconds:** Open the collection, replay the new discovery, and show its date and AI score. If personally reviewed, confirm it and show “Personally confirmed — not independently scientifically verified.” The original prediction stays unchanged.

**80–90 seconds:** Explain that Go, SQLite, and local BirdNET ONNX inference keep the loop inspectable and locally deployable. Expert-reviewed, opt-in evaluation data is future work.

## Screenshot and demo guidance

Capture the recording/upload state, a real inference result with visible candidate details, the collection entry with playback, and the quest showing its actual counts. Prepare a real mono 32 kHz, 16-bit PCM WAV clip with `ffmpeg -i input-audio.ext -vn -ac 1 -ar 32000 -c:a pcm_s16le -t 60 demo-bird.wav`, and run `make verify-inference AUDIO=/absolute/path/demo-bird.wav` before the 90-second demo so the model can download and warm up. Ensure the clip is at least one second long. No official sample download was verified in the restricted build environment. If model assets cannot be downloaded in the demo environment, show the interface and validation behavior but label inference unverified; do not present fixture candidates as real identifications. Avoid screenshots that expose local paths, private recordings, or personal data without consent.

## Validation disclosure

At initial preparation, Python adapter tests and the Go test suite passed, but sandbox DNS failure prevented installing BirdNET and downloading model assets. The sandbox also denied local socket access for a running-server browser walkthrough. No real species identification was observed. See [validation.md](validation.md) for checks and the manual end-to-end procedure.
