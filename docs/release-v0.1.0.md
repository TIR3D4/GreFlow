# GreFlow v0.1.0

Initial Go CLI for one IPv4 GRE tunnel per Linux host with entry/exit roles, TCP/UDP port mapping, identity ranges, owned iptables chains, systemd persistence, backups and best-effort rollback, repair, health, diagnostics and traffic counters.

Includes English/Persian documentation, root menu, amd64/arm64 static binaries, SHA256 checksums and a project-owned installer. Unit/race/vet, shellcheck and real-kernel network namespace integration tests gate this release.

GRE is unencrypted and uses IP protocol 47. Requires root, systemd, directly assigned IPv4 endpoints and provider GRE support. High-load, provider-specific and real VPS reboot validation remain deployment responsibilities. Native nftables, multi-instance support, signed artifacts and crash-resumable journaling are planned. Automated tuning is advisory only.

Installation: see README. Assets: greflow-linux-amd64, greflow-linux-arm64, SHA256SUMS.
