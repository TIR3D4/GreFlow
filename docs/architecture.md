# Architecture and scope

Go 1.22+ with no external module dependencies. Linux system tools do the kernel work; all invocations have a 20-second deadline and use argument arrays. A root-owned flock prevents concurrent mutations. One tunnel is supported per host.

| Module | Responsibility |
|---|---|
| cmd/greflow | Interactive menu, setup flags and commands |
| core/config | Strict JSON decoding, validation and atomic 0600 replacement |
| core/system | Bounded command runner, injectable for tests |
| core/tunnel | GRE lifecycle with ownership alias |
| firewall | Backend interface; iptables implementation and rule generation |
| core/manager | Backups, lifecycle, sysctl ownership, rollback, persistence, logs |
| core/health | Independently measured status, diagnostics, stats, tuning advice |

JSON replaces the suggested TOML paths in v0.1 to avoid a custom TOML parser or third-party dependency. Port mappings live in the same atomic configuration. Kernel-only configuration remains declarative; state stores the last applied configuration, sysctl originals, timestamp and explicit user verification.

The backend interface exposes Apply/Remove/Check; native nftables can implement it later. `iptables-nft` works through the iptables frontend today. Neither native nftables support nor multi-instance safety is claimed.

NAT hooks apply only to the entry's configured IPv4. Identity ranges use DNAT to an IP without specifying a port, preserving the original port. SNAT makes the exit observe the entry GRE address instead of the original client address. Reverse NAT uses conntrack. INPUT on the exit is limited to configured destination ports and GRE peer; the service must bind a reachable address.

Forwarding accepts related ICMP for path-MTU feedback and clamps TCP MSS on the GRE direction. UDP still needs an appropriate MTU/application behavior. No global firewall policy is changed. Rules in reserved chains must carry the GreFlow marker; foreign rules cause refusal rather than deletion.

v0.1 is a first release, not a claim of universal production validation. The automated real-kernel tests use namespaces; physical/provider GRE filtering, high-load behavior and reboot on a specific VPS still need deployment validation.
