# GreFlow v0.2.0

One entry server can now forward different public ports to multiple independent exit servers.

Use `greflow --tunnel NAME setup ... --network UNIQUE_PRIVATE_/30`, then `--tunnel NAME add/test/status/repair/restart/uninstall`. Each named instance owns its GRE interface, hashed firewall chains, config/state, backups and systemd service. `greflow tunnels` and menu selection show/manage instances.

The existing v0.1 configuration remains default with the same paths/alias/service and commands. Installer upgrades only the binary, preserves configs and does not restart live tunnels. No migration is required. Review the README for adding a second exit and moving a public port between tunnels.

Cross-instance resource and public-port collisions are rejected. A shared lock serializes startup; stopping/removing one instance leaves sibling rules/interfaces and forwarding intact. The last entry restores saved global sysctl originals. Integration tests cover real TCP/UDP paths to two exits, cleanup order, conflict rejection, concurrent startup and the original rollback/range regressions. CI also verifies the release installer.

This selects destinations by public port; it does not load balance or automatically fail over. GRE remains unencrypted and unauthenticated, and provider protocol-47/reboot/high-load validation is separate. Native nftables and crash-resumable journaling remain planned. Linux amd64/arm64 binaries and SHA256SUMS are included.
