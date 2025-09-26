# Repository Guidelines

## Project Structure & Module Organization
Core code lives in `src/`: `api` handles REST clients and stream helpers, `components` contains UI such as `chat/ChatExperience.tsx`, while `context`, `hooks`, and `utils` provide reusable logic. Static assets stay in `public/`, and Vite/Tailwind/TypeScript configuration sits in the repository root. Production bundles land in the git-ignored `dist/` directory—never edit generated files directly.

## Build, Test, and Development Commands
- `npm install` – install dependencies after cloning or dependency updates.
- `npm run dev` – start the Vite dev server (default http://localhost:5173) against the configured backend.
- `npm run build` – type-check and emit optimized assets into `dist/`.
- `npm run preview` – serve the production bundle for smoke checks.
- `npm run lint` – run ESLint with the shared configuration; resolve warnings before submitting.

## Coding Style & Naming Conventions
Write new code in TypeScript and React function components. Follow the two-space indentation already used in `src/App.tsx`. Name components and contexts in PascalCase (`PersonaSidebar`), helpers in camelCase, and hooks with a `use` prefix. Favor Tailwind utility classes and the design tokens defined in `tailwind.config.js`. Run `npm run lint` regularly to keep formatting and hook usage consistent.

## Testing Guidelines
Automated tests are not bundled yet; when introducing new behavior, add minimal coverage next to the affected code (for example, `ComponentName.test.tsx`) and document its execution in the PR. At minimum, smoke-test the chat flow with `npm run dev`, confirming persona loading, session creation, streaming updates, and speech functionality when applicable.

## Commit & Pull Request Guidelines
Repository history is not visible in this checkout, so default to Conventional Commits (e.g., `feat: improve speech health polling`, `fix: guard stream controller cleanup`). Keep commits focused on a single concern. Pull requests should provide a concise summary, linked issues, screenshots for UI updates, note any backend prerequisites, and confirm `npm run lint` plus `npm run build` results.

## Environment & Configuration Tips
Set `VITE_API_BASE_URL` in `.env.local` for non-default backends; the fallback is `http://localhost:8080/api`. Tailwind theming relies on the custom tokens under `theme.extend.colors` and `backgroundImage`, so reuse those instead of hard-coding values. The selected theme persists via the `ztavern-theme` key—clear it when diagnosing display bugs.
