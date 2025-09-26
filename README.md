# Z Tavern - Liquid Glass Frontend

A React + TypeScript single-page app that powers the Z Tavern "Liquid Glass" experience: persona-driven chat, live streaming replies, and full duplex speech.

## Highlights
- **Persona library** – fetch personas from the backend, search by keyword or tags, shuffle for inspiration, and inspect rich profiles.
- **Streaming conversations** – create sessions, watch assistant deltas arrive over Server-Sent Events, and preserve context per persona.
- **Speech pipeline** – capture microphone audio, down-sample and encode to PCM16, stream over WebSocket for ASR, and request TTS playback on demand.
- **Glassmorphism UI & theming** – Tailwind tokens define the Liquid Glass look; a ThemeProvider keeps dark/light mode in sync with local storage.

## Quick Start
1. Install Node.js 20 or newer.
2. Install dependencies: `npm install`
3. (Optional) create `.env.local` and set `VITE_API_BASE_URL` if your backend is not `http://localhost:8080/api`.
4. Run the dev server: `npm run dev`
5. Visit the printed URL (default http://localhost:5173). Ensure the backend API is reachable; otherwise persona loading and streaming will fail.

## Environment & Backend Contracts
The frontend expects the backend to expose the following endpoints beneath `VITE_API_BASE_URL`:
- `GET /personas` – list available personas.
- `POST /session` – create a conversation session for a persona.
- `POST /messages` – enqueue a user message.
- `GET /stream/:sessionId` – Server-Sent Events stream delivering `start`, `delta`, `end`, and `status` payloads.
- `POST /speech/transcribe` and `POST /speech/transcribe/:sessionId` – ASR entry points for freeform and session-bound transcription.
- `POST /speech/synthesize` and `POST /speech/synthesize/:sessionId` – TTS requests returning either JSON metadata or audio blobs.
- `GET /speech/health` – speech service readiness probe.
- `GET /speech/socket` (WebSocket) – bidirectional speech stream used by live ASR/TTS.

Grant microphone permission when testing speech locally. Browsers typically require `https://` or `http://localhost` for audio capture.

## Available Scripts
- `npm run dev` – start Vite with hot module replacement.
- `npm run build` – type-check via `tsc -b` and produce an optimized bundle.
- `npm run lint` – run ESLint across the repository.
- `npm run preview` – serve the production build locally.

## Project Structure
- `src/api` – REST, SSE, and speech client helpers (`apiClient`, `createStreamController`, `createSpeechSocket`).
- `src/components/chat` – persona sidebar, conversation timeline, composer, and profile panels.
- `src/hooks/useChatOrchestrator.ts` – central state management for personas, sessions, streaming, and speech states.
- `src/context` – theme context/provider.
- `src/utils/persona.ts` – persona helpers for prompts, tags, initials, and mood metrics.
- `public/` – static assets. Production bundles emit to `dist/` (git ignored).

## Development Workflow
- Keep edits in TypeScript/React function components; follow the existing two-space indentation.
- Reuse Tailwind utilities and the `ztavern-*` tokens declared in `tailwind.config.js`.
- Prefer calling the utilities exported from `src/api` instead of mixing in ad-hoc `fetch` logic.
- When adding new API payloads, extend `src/api/types.ts` and adjust `ConversationState` where required.
- Before opening a PR, run `npm run lint` and `npm run build`, then manually verify persona loading, message streaming, speech recording, and TTS playback.

## Troubleshooting
- **No personas appear** – confirm the backend is reachable and the API base URL is correct.
- **Streaming stalls** – check `createStreamController` listeners, browser devtools network tab, and backend SSE headers.
- **Speech errors** – inspect the speech status banner, verify microphone permission, and check `/speech/health` results.
