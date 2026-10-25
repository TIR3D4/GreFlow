# GreFlow Console — interactive design reference

This is a **demo-only UI and Codex implementation handoff**, not a working server-management backend. It makes no network management calls and does not mutate tunnels. It intentionally does not generate executable installation commands or collect credentials. All actions use in-memory fixtures. Reload resets changes; only theme preference is persisted.

Run on Windows or Linux with Node.js installed:

```sh
node web/console/serve.cjs
```

Open `http://127.0.0.1:8787`. Or open `index.html` directly in a modern browser; the copy button may be unavailable under `file:`. No npm install, bundler or CDN is needed. Port override: `GREFLOW_PREVIEW_PORT=8788` (PowerShell: `$env:GREFLOW_PREVIEW_PORT='8788'`).

Included flows: overview, servers, tunnel cards/table/search/filter, detail, simulated probes/start/stop/restart/delete, port edits with collision checks, transfer with simulated rollback, four-step creation wizard, operation history, schedules, theme settings and demo JSON export/reset. Keyboard-accessible native dialogs, responsive layouts, Persian RTL and LTR technical fields. Some production-only capabilities are explicitly disabled with an adjacent reason.

```sh
node --check web/console/app.js
node --test web/console/domain.test.cjs
```

See `docs/console/CODEX-HANDOFF.fa.md` for the full implementation prompt, `PRODUCT.fa.md` for UX, `ARCHITECTURE.md` for backend/API/security and `ACCEPTANCE.md` for release gates. This preview does not satisfy those production gates.

The bundled Vazirmatn font is licensed under SIL OFL; see `assets/OFL.txt`. Reference screenshots under `docs/console/references/` are generated design examples, not measured telemetry.
