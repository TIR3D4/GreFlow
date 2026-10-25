# Release gates and evidence

The console is currently a design handoff, not a released remote-management product. Mark gates complete only with reproducible test evidence, not merely implementation claims.

## Existing behavior

- [ ] Existing Go unit/race/vet and network-namespace integration suites pass.
- [ ] No change to default/named v0.2 config paths, rule tags or CLI behavior without explicit migration tests.
- [ ] Import existing active default + 12 named tunnels without recreating interfaces, dropping traffic or losing mappings.
- [ ] Detect multi-IP host identity, duplicate endpoints/subnets/ports and stopped-instance reservations.
- [ ] No blanket iptables/nft/conntrack flush, global snapshot restore or unrelated Docker/Xray changes.

## Real end-to-end behaviors

- [ ] Windows loopback launch, owner bootstrap, strict SSH host key onboarding, discovery and remote scheduled test work.
- [ ] Linux Compose/systemd install, HTTPS, enrollment, token expiry/reuse/revocation, restart and data restore work.
- [ ] Add GRE against controlled Linux namespaces/VMs, install peer, observe H2/H3, verify actual client separately.
- [ ] Add/remove TCP/UDP ports, detect conflict/range restrictions, preserve other instances.
- [ ] Stop/start/restart/reset counter handling tested; service active-exited not misclassified.
- [ ] Transfer 2021 with 2020 traffic active; new target preflight failure leaves old path untouched.
- [ ] Failure between source removal and target addition, control-plane restart and agent disconnect: recover or mark uncertain, never false success.
- [ ] Rollback failure shown explicitly with actionable recovery evidence.
- [ ] Existing conntrack behavior documented; optional selective session removal cannot affect other ports/hosts.
- [ ] No shell injection via address, name, protocol, network, path, schedule, token or action.
- [ ] Remote schedule still runs while Windows app is closed; install/edit/disable/delete reconcile correctly.
- [ ] Client reconnect resumes operation/events without duplicate mutation.
- [ ] Shared host lock and simultaneous UI/CLI changes produce revision conflict or serialized safe update.

## UI and accessibility

- [ ] All nav sections, forms, dialogs, context actions and disabled states have actual behavior or a precise capability reason.
- [ ] 360/390/768/1280/1440px layouts tested; no body horizontal overflow; technical table scroll is deliberate.
- [ ] Light/dark/system preference, no external font requirement, connected Persian glyphs and isolated LTR strings.
- [ ] Keyboard-only traversal, visible focus, modal focus trap/return, Escape, screen reader labels and field-linked errors.
- [ ] Touch targets >=44px, AA contrast, 200% zoom, reduced motion, no color-only status.
- [ ] Loading, empty, filtered empty, error, offline, stale, permission, unsupported, partial and rollback states exercised.
- [ ] Counts match filtered dataset; truncated mappings accessible; no false healthy when monitoring stale.
- [ ] Real logs can be copied safely; no secret or bidi controls injected into copied commands.

## Security, operations and distribution

- [ ] Bootstrap/login/TOTP/recovery/session revocation and per-object RBAC tested.
- [ ] CSRF, Origin, replay, enrollment brute force, rate limit and token redaction tests pass.
- [ ] Keys encrypted with documented OS/server key wrapping, loss/recovery and encrypted backup procedure.
- [ ] Backup is consistent under writes; restore on a clean host tested, with encrypted credential recovery if included.
- [ ] Audit append history, metrics retention, storage exhaustion and failure boundaries tested.
- [ ] Production build has no demo provider, fixture statistics, simulated successes or placeholder installer URL.
- [ ] Installer artifact versions pinned, checksums/signing/SBOM/licenses included.
- [ ] Linux amd64/arm64 + Windows supported targets build in CI; smoke tests documented.
- [ ] Upgrade and rollback preserve user data/tunnels; uninstaller scope explained.
- [ ] Release notes distinguish implemented providers/capabilities; no unimplemented failover or bypass guarantees.

## Current handoff checks

Run `node --test web/console/domain.test.cjs` and `node --check web/console/app.js`.
The dependency-free preview performs no SSH, firewall, systemd or networking changes.
