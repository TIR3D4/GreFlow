# Handoff verification — 2026-10-25

Scope: the HTML/CSS/JavaScript design reference and documentation. This is not evidence for a production backend or live server changes.

Environment: Node v24.22.1, Playwright 1.62.1, headless Chromium 153.0.8010.0 (temporary QA dependency). Bundled local Vazirmatn loaded successfully.

Passed:

- `node --check web/console/app.js`, `domain.js`, `serve.cjs`, `browser.test.cjs`.
- Four `node:test` domain tests: malformed IPv4/ports, per-host/per-protocol reservations, name/subnet allocation and wizard conflicts.
- One browser integration test covering all navigation, card/table switch, search, simulated test feedback, modal Escape, staged creation, rejection of an occupied port, enrollment step gating, simulated rollback and successful mapping transfer, schedule creation, theme toggle and mobile server creation.
- No uncaught browser `pageerror` during tested flows.
- No document horizontal overflow at 360, 390, 768, 1280 or 1440 CSS pixels.
- Manual visual inspection of desktop and mobile browser captures; corrected off-canvas sidebar and shrinking traffic graph overflow.
- `git diff --check`.

Not run / not claimed:

- Live SSH, real agent enrollment, kernel GRE/firewall mutation, real port transfer, scheduler execution or notifications: no backend exists in this handoff.
- Go unit/integration suites: no Go runtime available; existing Go/core/installer source unchanged.
- Windows native executable, Linux production distribution, accessibility AA audit, screen reader testing, login/TOTP and encrypted storage: release implementation gates, not implemented.
- GitHub remote update: read access succeeded; push dry-run failed because HTTPS write credentials were unavailable. Work is committed locally on `feat/web-console-design`.

Reproduction:

```sh
node --test web/console/domain.test.cjs
node --check web/console/app.js
node web/console/serve.cjs
```

Optional browser QA with locally installed Playwright and its Chromium:

```sh
npm install --prefix /path/to/qa-tools playwright@1.62.1
/path/to/qa-tools/node_modules/.bin/playwright install chromium
# Make that node_modules available through NODE_PATH, or install in your QA environment.
node --test web/console/browser.test.cjs
```

`CHROMIUM_MODULE` can point to an ESM default export compatible with Sparticuz Chromium for restricted environments. It is a QA-only option. `GREFLOW_QA_DIR` optionally saves screenshots; they are not required for normal preview use.
