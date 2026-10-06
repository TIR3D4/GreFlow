# Changelog

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
