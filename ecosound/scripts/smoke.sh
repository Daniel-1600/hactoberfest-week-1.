#!/usr/bin/env bash
# Local API smoke check. Passing a recording exercises real BirdNET inference and saves one discovery.
set -euo pipefail

base_url="${ECOSOUND_URL:-http://127.0.0.1:8080}"
audio_file="${1:-}"
command -v curl >/dev/null
command -v python3 >/dev/null

health="$(curl -fsS "$base_url/health")"
printf '%s\n' "$health" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["status"]=="ok"; print("health: ok; inference configured:", d.get("inference"))'

if [[ -z "$audio_file" ]]; then
  printf '%s\n' "Pass a 1–60 second bird audio file to test real identification and save one discovery."
  exit 0
fi
if [[ ! -f "$audio_file" ]]; then
  printf '%s\n' "Audio file does not exist: $audio_file" >&2
  exit 2
fi

result="$(curl -fsS -F "audio=@$audio_file" "$base_url/api/identify")"
printf '%s\n' "$result" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["model"]=="birdnet"; print("recording:", d["recording_id"], "candidates:", len(d["predictions"]))'
prediction_id="$(printf '%s\n' "$result" | python3 -c 'import json,sys; rows=json.load(sys.stdin)["predictions"]; print(rows[0]["id"] if rows else "")')"
if [[ -z "$prediction_id" ]]; then
  printf '%s\n' "No detection; upload and health checks passed, no discovery saved."
  exit 0
fi

discovery="$(curl -fsS -H 'Content-Type: application/json' -d "{\"prediction_id\":$prediction_id}" "$base_url/api/discoveries")"
printf '%s\n' "$discovery" | python3 -c 'import json,sys; d=json.load(sys.stdin); assert d["prediction_id"] > 0; print("saved discovery:", d["id"], d["scientific_name"])'
curl -fsS "$base_url/api/quests/today" | python3 -c 'import json,sys; d=json.load(sys.stdin); print("quest:", d["date"], ", ".join(str(t["current"])+"/"+str(t["target"]) for t in d["tasks"]))'
