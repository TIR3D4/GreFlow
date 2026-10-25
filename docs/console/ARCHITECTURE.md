# GreFlow Console: implementation contract

Status: proposed release architecture; not an implemented backend. Existing CLI is v0.2.0. The browser reference is a separate explicit demo.

## Repository layout and compatibility

Keep `cmd/greflow`, `core/config`, `core/manager`, `core/health`, `core/tunnel`, `core/system`, `firewall`, existing install paths and release assets compatible. Add, do not replace:

```
cmd/greflow-console/          # Windows/Linux control plane
cmd/greflow-agent/            # Linux typed dispatcher + local scheduler
internal/console/            # API, auth, jobs, inventory, metrics, audit
internal/agent/              # protocol, enrollment, policy, probes
internal/providers/gre/      # adapts existing manager, no shell strings
internal/providers/          # interface for future implementations
web/console/                 # current runnable visual reference
web/app/                     # production frontend after implementation
packaging/                   # Windows zip, systemd, Compose, reverse proxy
docs/console/                # product, API, release gates, Codex handoff
```

Go for control plane/agent to share typed validation with the current core. Frontend HTML/CSS/JavaScript (TypeScript acceptable), reusable view components; React is a reasonable implementation option, not a dependency of the supplied demo. No Java/JVM requirement: JavaScript is the intended browser language. Ship built static files embedded in the control-plane binary. Vendor fonts. Pin dependency and runtime versions, record licenses, generate SBOM and checksums.

Do not import Linux-only code into the Windows control plane. The current CLI contains root checks, systemctl and kernel operations; isolate provider execution in Linux agent. Cross-compile artifacts and test both platforms in CI.

## Execution modes

- Windows: single executable, local data directory, listens on loopback by default, bootstrap owner, optionally opens browser. SSH management is useful for local mode because Iran cannot reach a localhost agent endpoint. Native secure keystore for wrapping the encryption key. Closing browser does not kill operations; closing process interrupts central monitoring, not remote GRE or local server schedules.
- Linux: binary + systemd or Compose, persistent volume, non-root control plane, TLS reverse proxy. Remote agent connects outbound over mutually authenticated TLS; enrollment uses an expiring scoped single-use credential. Network or proxy TLS errors must fail closed.
- HTTPS-reachable endpoint required for push-agent mode. Local Windows users cannot assume public reachability; offer SSH polling first, or explicitly configure a reachable TLS endpoint. Do not silently create a public tunnel.
- SSH mode: strict known-host verification, bounded commands, typed request executable on target. Never concatenate user strings into shell. Avoid arbitrary uploaded script/terminal endpoints. Explicit ownership of installed helper and removal.

## Data model

Use stable UUIDs independent of user-editable names/IPs. Suggested tables:

`users`, `roles`, `sessions`, `totp_recovery_codes`, `hosts`, `host_addresses`, `connections`, `credentials`, `agents`, `enrollment_tokens`, `tunnels`, `mappings`, `observations`, `metric_samples`, `operations`, `operation_steps`, `resource_leases`, `snapshots`, `schedules`, `schedule_runs`, `notification_destinations`, `incidents`, `audit_events`, `schema_migrations`.

SQLite with transactional migrations, foreign keys, busy timeout and WAL is suitable for a single control-plane instance; use a supported driver that can build for Windows and Linux and verify backup behavior. Distributed HA is out of scope for v1; never run multiple writers against the same SQLite volume across hosts. Store bounded history; aggregate metrics and rotate logs.

Credential blobs encrypted at rest; wrapping key not adjacent unprotected to database backups. Backups must document key recovery, permissions and exclusion options. No browser localStorage credentials; theme/filter preferences are okay.

## Provider contract

Provider exposes `Discover`, `Capabilities`, `ValidatePlan`, `Probe`, `Apply`, `Observe`, `Stop`, `Restart`, `Remove`, `Snapshot`, `Restore`, `Schedule`. Requests are typed and authorized. Responses have evidence/timestamps, not a single boolean.

GRE capabilities from current engine:

- IPv4 direct endpoints; no transparent NAT-to-private guarantee.
- unkeyed endpoint pair cannot repeat on the same host; map many ports to one link.
- private aligned /30; unique names, interfaces, subnets across host configs.
- MTU 576..1476; coordinated both sides.
- TCP single-port probe; UDP and ranges need application-aware verification.
- identity port ranges only; TCP and UDP reservations distinct.
- config paths default `/etc/greflow/config.json`, named `/etc/greflow/tunnels/NAME/config.json`.
- oneshot `active (exited)` can be normal. Config reserved even when inactive.
- `apply`/`repair` recreate interface and can interrupt sessions/reset counters.
- endpoint/role/network edits require lifecycle migration; uninstall of last instance may remove CLI binary. Never invoke uninstall for a simple port edit; inventory and dependency restoration must be explicit.

Add a structured read-only CLI output/API for discovery and health; do not parse human Persian/English health strings as a permanent integration. Preserve old stdout format by default and add explicit `--json` or new adapter methods after tests.

## API shape

Use `/api/v1`. JSON field names snake_case. Error envelope:

```json
{"error":{"code":"PORT_CONFLICT","message":"...","field":"public_port","retryable":false,"operation_id":null,"details":{"owner_tunnel_id":"uuid"}}}
```

| Endpoint | Method | Behavior |
|---|---|---|
| `/bootstrap`, `/auth/login`, `/auth/totp`, `/auth/logout` | POST | bootstrap and session lifecycle; rate limited |
| `/session`, `/capabilities` | GET | identity, permissions and installed capabilities |
| `/servers` | GET/POST | inventory or connect/enrollment job |
| `/servers/{id}` | GET/PATCH | metadata only; address change creates plan |
| `/servers/{id}/discover` | POST | read-only discovery operation |
| `/servers/{id}/import-plan` | POST | diff existing configs before adoption |
| `/servers/{id}/observations` | GET | fresh/stale evidence with timestamps |
| `/servers/{id}/tunnels` | GET | filter/sort/paginate, include rollup |
| `/tunnel-plans` | POST | validate, allocate and lease host resources |
| `/tunnel-plans/{id}/apply` | POST | enqueue typed installation, return 202 |
| `/enrollments` | POST | scoped expiring credential, expose once |
| `/enrollments/{id}` | DELETE | revoke unused enrollment |
| `/tunnels/{id}` | GET | desired + observed + capabilities |
| `/tunnels/{id}/actions` | POST | typed test/start/stop/restart/repair/remove |
| `/tunnels/{id}/mapping-plans` | POST | preflight mapping edits |
| `/mapping-transfers` | POST | plan source/target and rollback policy |
| `/mapping-transfers/{id}/apply` | POST | revision-guarded transaction job |
| `/operations`, `/operations/{id}` | GET | filtered history and resumable progress |
| `/operations/{id}/cancel` | POST | only cancellable state transitions |
| `/events?cursor=...` | GET SSE | ordered events with cursor/resume; bounded replay |
| `/schedules` | GET/POST | remote host-persisted schedules |
| `/schedules/{id}` | PATCH/DELETE | reconcile generation and report host ack |
| `/backups`, `/restore-plans` | POST | encrypted snapshots, dry-run restore |
| `/notification-destinations` | GET/POST | redacted settings, scoped access |
| `/audit` | GET | permission-scoped, secrets redacted |

All mutation APIs require authenticated role, scope, CSRF protection (for cookie sessions), valid Origin and idempotency key. Return accepted operation ID immediately; operations execute server-side. Revision/ETag checks prevent overwriting external CLI changes. Agent endpoints separate from browser cookies. Never expose a generic shell command API.

Observation fields: `kind`, `status` (`ok|failed|unknown|unsupported`), `observed_at`, `expires_at`, `latency_ms`, `loss_percent`, `error_code`, `evidence`, `source`. Time is RFC3339 UTC in API; UI renders configured timezone. Distinguish host peer ICMP from GRE peer, TCP connect, protocol handshake and manual verification.

Tunnel response example:

```json
{"id":"uuid","server_id":"uuid","name":"exit11","method":"gre","revision":4,"desired":{"role":"entry","local":"94.182.151.153","remote":"66.92.161.146","network":"10.77.0.40/30","mtu":1476,"forwards":[{"protocol":"tcp","public":"2021","destination":"2021"}]},"observed":{"at":"2026-10-25T00:45:00Z","service":"active_exited","gre_peer":"ok","tcp_destination":"ok","user_verified":"unknown","drift":false},"freshness":"fresh"}
```

## Jobs, locks, transfer and recovery

Job states: queued, preflight, waiting_for_peer, running, verifying, succeeded, failed, cancelling, cancelled, uncertain, rolling_back, rolled_back, rollback_failed. Persist each transition. Only succeeded after readback/probes appropriate to action. No fixed progress percentages invented from timers; show stages or measured counts.

Per-host serialization + existing GreFlow host lock; define lock ordering and avoid holding a core lock across systemctl restart, which invokes GreFlow. Bounded deadlines and structured arguments on all process calls. Reserve names/interface/subnet/endpoint+protocol+port range across stopped configs. Consult real host route tables, external modifications and both endpoint configs, not just database.

Mapping transfer must journal previous configuration of both selected instances before mutation. Existing manager snapshots are instance-scoped and not a crash-safe cross-instance transaction. Implement recovery protocol before exposing production transfer. Check both host/app availability and desired revision; apply scoped operations; read back rules; probe a NEW connection. In failure restore original owned configs/rules; never replay a global firewall snapshot. Do not flush conntrack; optional scoped removal with explicit acknowledgement, bounded tuple selection and audit. If rollback uncertain keep job unresolved and offer recovery plan.

## Scheduler and monitoring

Agent/helper creates uniquely owned systemd units with strict validated IDs and metadata. Do not overwrite user crontabs. Central schedule state says pending until host acknowledges. Schedule removal deletes only owned unit files. Disable/maintenance and reboot behavior tested. Timers and local recovery continue with console offline; central notifications do not unless a separate notifier is configured.

Probe concurrency bounded by host/global budget. Add jitter/backoff. Metadata heartbeat every ~15s; active GRE checks default 60s, destination checks 60s configurable, never thousands per second. Freshness thresholds explicit (e.g. heartbeat stale >45s, health stale >180s) and configurable. Continuous ping is not application confirmation. Counter deltas use monotonic elapsed time, detect reset/boot/interface generation and preserve cumulative accounting by separate series.

## Security and deployment

Control plane least privilege; root-only operations constrained to Linux agent with typed allowlist and exact ownership. Agent request replay protection, mTLS renewal/revocation and signed/verified upgrades. SSH identities pinned, permissions restricted. Authentication cookies HttpOnly/Secure/SameSite; CSRF; CSP and no inline production scripts; security headers and dependency audit. No token in URL query logging. Redact stdout/stderr, secrets, config snippets and exports. Permission-check each server/job object, not just navigation visibility.

Package Windows amd64/arm64 only when supported/tested and Linux amd64/arm64. Console installer distinct from existing CLI installer; updating panel cannot restart production tunnels silently. No published release/tag until acceptance gates pass. Include uninstall preserving server tunnels by default and explicit separate remote cleanup.
