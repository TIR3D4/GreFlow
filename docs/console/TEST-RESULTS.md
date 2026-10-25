# Validation evidence — Console 1.0.0

This ledger distinguishes executed tests from release gates and remaining limits.
The authoritative CI results are attached to the exact release commit in
[GitHub Actions](https://github.com/TIR3D4/GreFlow/actions/workflows/console.yml).
A release is created only after both Windows and Linux jobs succeed.

## Executed locally on Windows amd64 (2026-10-25)

- Go 1.27.3: `go test -p 2 ./...` and `go vet ./...` passed. Existing core,
  CLI and firewall tests remain included. Linux provider tests cross-compile.
- Real loopback SSH: pinned host keys, password/private-key authentication,
  changed-key refusal, root/sudo command boundaries and sensitive stderr suppression.
- Automatic helper preparation: existing helper preservation, amd64/arm64 selection,
  incompatible version, unsupported OS/architecture, invalid package, permissions,
  upload/disk/checksum failures, concurrent install refusal, lost connection and
  refusal to mutate when progress cannot be persisted. Operator install is refused.
- Auth: password-only setup, bcrypt, CSRF, object scope, role changes, session
  invalidation, password rotation and legacy TOTP retirement without losing users.
- Durable operations: idempotent retry, conflict rejection and uncertain results
  after transport loss. Interrupted work is not blindly executed again.
- Encrypted backup: tampering/wrong password refusal, full control-plane restore
  onto a clean instance, vault re-encryption, session revocation and no queued replay.
- Real loopback mTLS: one-use enrollment, required client certificate, queue scope,
  revocation. These tests do not use production credentials.
- Browser tests use the real Windows executable for setup, login, navigation,
  empty inventory, account editing, backup download and restore preview.
  Explicit UI-only fixtures separately exercise 27 tunnel cards, filtering,
  pagination, card/table modes, outside IP/flags/Check-Host links, stale evidence,
  preparation reports, report downloads and invalid reboot confirmation.
- Keyboard focus/Escape, local font, light/dark and horizontal overflow checks at
  widths 360, 390, 768, 1280 and 1440. Screenshots are QA artifacts, not live evidence.
- Check-Host parser: exact IP matching, country conflicts, public-address filtering,
  host scopes, exit-role address selection, caching and stale data after HTTP 429.
- Fixed server command construction and credential/token redaction are unit tested.
- Client confirmation retains its original timestamp/revision/generation. Discovery
  does not refresh it; old CLI confirmations without timestamps stay unknown.
- Timezone and overnight maintenance boundaries are unit tested.

## Authorized live checks

With the owner's explicit automatic-install request, the bundled helper was installed
on two existing Linux amd64 SSH hosts and read-only discovery succeeded on both.
The installation did not replace the engine, tunnel configurations, firewall rules,
sudo policy or services. No production reboot or destructive tunnel test was executed.
Host addresses, credentials, databases and live reports are excluded from release assets.

## Release CI gates

The workflow runs Go tests/vet on Windows and Ubuntu 24.04, Linux race tests,
real browser tests on both platforms and isolated Linux network-namespace integration
for GRE, TCP/UDP forwarding and transfer/rollback. It builds Windows amd64 and Linux
amd64/arm64 bundles. A failed gate blocks automatic publication.
The initial release implementation passed all Windows/Linux console jobs and the
CLI kernel suite in [PR #1](https://github.com/TIR3D4/GreFlow/pull/1), including
[console run 40162800933](https://github.com/TIR3D4/GreFlow/actions/runs/40162800933).
Later UI refinements must pass those gates again on their own commit.
Browser regressions also cover preserving expanded server tools/keyboard focus during
polling, Escape cleanup and distinguishing never-run checks from expired results.

Cross-compilation alone is not Linux runtime evidence. The workstation has no Linux
kernel/WSL/Docker daemon; use the linked CI run to establish Linux gate results.

## Scope and remaining limits

- GRE is not encrypted or authenticated. Application encryption is independent.
- Transfer supports one TCP mapping on the same entry endpoint. Cross-entry migration,
  UDP/range transfer, short-lived resource reservations spanning both peers and
  remote endpoint migration are outside version 1.
- Backup restores the control plane, including encrypted credentials, CA, accounts,
  inventory, schedules and history. It does not restore remote host configurations.
  A restored CA requires a console restart. Existing work is not replayed.
- Resource information is an on-demand snapshot. Historical CPU/memory/disk charts,
  traffic reset/rate series, DNS A/AAAA checks, tags and configurable audit retention
  are not implemented. Retain/rotate the host's own system logs separately.
- Daily maintenance windows use their configured timezone. Recovery shares the
  host lock and a persisted limit of six attempts/hour plus five-minute cooldown.
  Full power-loss, disk-full and calendar/systemd testing across distributions remains
  a deployment validation task; unit tests are not substitutes for those scenarios.
- Linux arm64 is cross-compiled, not tested on physical arm64. Compose, public HTTPS,
  distribution-specific package rollback and production reboot recovery are not
  covered by the local validation. Primary Linux CI target is Ubuntu 24.04 amd64.
- Keyboard/responsive browser checks are not a complete WCAG or screen-reader audit.
- Bundles include SHA256 checksums, dependency licenses and a CycloneDX SBOM.
  They do not have independent cryptographic release signatures.
- Check-Host location is approximate and may disagree between databases. Its
  ISP/Org field is not proof of the physical datacenter. Failed refreshes preserve
  the original observation time and explicitly mark cached information stale.

## Compatibility and packaging

The original CLI remains v0.2.0; Console has a separate `CONSOLE_VERSION`.
Installed config paths, aliases and service names remain compatible. Linux flock
was moved into a platform-specific file; JSON CLI commands are additive.
The automatic installer uploads bundled bytes over SSH and checks SHA256/protocol
before atomic no-overwrite publication. It does not fetch a script from the Internet.
Existing incompatible helpers require deliberate operator intervention.

The final executable embeds `web/app`, with no demo inventory or simulated transport.
`web/console` is an explicitly separate design reference and test fixture, never served
by the production binary. Windows/Linux bundles include both helper architectures,
launch/install scripts, Persian tutorials, troubleshooting, licenses and checksums.
