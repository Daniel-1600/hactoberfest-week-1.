# EcoSound frontend

Next.js App Router, TypeScript, Tailwind CSS, and Lucide. The application calls the Go API; it does not bundle mock predictions or sample discoveries.

## Run

Requires Node.js 22.12+ and the EcoSound backend, database, FFmpeg, and BirdNET configured as described in the repository README.

```sh
npm ci
cp .env.example .env.local
npm run dev
```

The project `.npmrc` keeps optional test-tool peer resolution consistent with the lockfile. This avoids an Arborist peer-resolution error in the environment’s npm 10.9.7.

Open `http://localhost:3000`. `NEXT_PUBLIC_API_URL` defaults to `http://localhost:8080`; set it to a browser-reachable API origin before building when running on another machine. The backend must allow the frontend origin through CORS.

Microphone capture works on HTTPS or localhost and requests permission only when you select Start recording. Audio file upload provides an alternative. Recordings automatically stop at 60 seconds; both recording and upload are limited to 15 MiB. The backend validates decoded duration and actual audio content. Playback uses the captured Blob and the stored WAV returned by the backend.

Only server results appear as candidates. A saved recording earns quest credit once, and quest/collection state is fetched again after saving. Personal confirmation is a user annotation and does not establish scientific verification.

## Check

```sh
npm run typecheck
npm run lint
npm test
npm run build
```

Tests mock browser capture and API responses to exercise microphone permission, cleanup, automatic stopping, real multipart submission, saving, duplicate protection, quest refresh, no detection, server errors, confirmation wording, and defensive metadata handling. They do not validate a physical microphone or the BirdNET model.

## Managed environment verification

Dependencies were installed and `npm ci` verified offline using an existing Linux x64 package cache. The lockfile has ordinary registry URLs and integrity values. Unused compiler packages for other platforms were not available in that cache; an online `npm install` can refresh optional platform metadata when needed.

The build succeeds in this Linux environment. Next.js attempts to repair missing metadata for unused SWC platforms and prints a non-fatal error because the sandbox blocks its shell subprocess. With the Linux compiler installed, `NEXT_IGNORE_INCORRECT_LOCKFILE=1 npm run build` skips that repair during offline verification. Local listening sockets are blocked here, so browser visual checks and physical microphone capture remain unverified.
