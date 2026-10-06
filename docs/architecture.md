# Architecture and scope

Go 1.22+ with no external module dependencies. Linux system tools do the kernel work; all invocations have a 20-second deadline and use argument arrays. A root-owned host-wide flock serializes mutations, with bounded waiting for concurrent service starts. Multiple independent named tunnels are supported; commands without a name target the legacy default.

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

The backend interface exposes Apply/Remove/Check; native nftables can implement it later. `iptables-nft` works through the iptables frontend today. Native nftables support is not claimed. Named instances use stable hashed chain names, instance-specific rule comments and link aliases; default ownership names stay compatible with v0.1.

NAT hooks apply only to the entry's configured IPv4. Identity ranges use DNAT to an IP without specifying a port, preserving the original port. SNAT makes the exit observe the entry GRE address instead of the original client address. Reverse NAT uses conntrack. INPUT on the exit is limited to configured destination ports and GRE peer; the service must bind a reachable address.

Forwarding accepts related ICMP for path-MTU feedback and clamps TCP MSS on the GRE direction. UDP still needs an appropriate MTU/application behavior. No global firewall policy is changed. Rules in reserved chains must carry the GreFlow marker; foreign rules cause refusal rather than deletion.

v0.1 is a first release, not a claim of universal production validation. The automated real-kernel tests use namespaces; physical/provider GRE filtering, high-load behavior and reboot on a specific VPS still need deployment validation.

Named paths: `/etc/greflow/tunnels/NAME/config.json`, `/var/lib/greflow/tunnels/NAME/state.json`; service `greflow-NAME.service`; interface default `gf-NAME`. Config scanning validates interface, private-network, unkeyed endpoint-pair and entry public-port conflicts before host changes. Desired inactive configurations also reserve their resources for restart.

The first entry captures pre-GreFlow global sysctl originals. Later active entries inherit those originals; stopping one leaves shared kernel settings while another entry is active. The last entry restores values only when still equal to GreFlow's writes. Shared host mutation is serialized; state remains subject to the documented crash-recovery limitations. GRE endpoint pairs must be distinct because v0.2 does not configure GRE keys.
