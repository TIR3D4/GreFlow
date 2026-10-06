# Changelog

## 0.2.0

- Named tunnels via --tunnel NAME, with independent config/state, interface aliases, owned chains and systemd services.
- One entry can forward different public ports to multiple exit servers.
- Preserve v0.1 default paths, aliases and CLI commands; no config migration.
- Validate cross-instance interface, subnet, endpoint-pair and public-port conflicts.
- Share global kernel originals safely; restore only after the last entry stops.
- Serialize concurrent startup with bounded host-wide lock waiting.
- Instance-scoped uninstall retains the binary/configs needed by other tunnels.
- Add menu selection and tunnel inventory, two-exit real-kernel tests and regression coverage.
- Refresh traffic counters after health probes.

## 0.1.1

- Deduplicate shared SNAT/filter rules when multiple public ports use one destination.
- Expand real-kernel tests for shared destinations and automatic rollback after a firewall failure.
- Count forwarded conntrack entries using original public and translated reply tuples.
- Read release version from VERSION and skip stale-commit publication.

## 0.1.0

- Typed Go CLI and root menu; private IPv4 /30 GRE entry/exit setup.
- TCP/UDP mappings, multiple ports and identity ranges.
- Scoped owned firewall chains, duplicate-hook repair and safe teardown.
- Atomic JSON config/state, process locking, structured logs and recovery backups.
- systemd persistence, H0–H6 diagnostics, stats and advisory tuning.
- Unit tests, real-kernel integration tests, CI and amd64/arm64 release builds.
- English/Persian documentation and MIT license.
