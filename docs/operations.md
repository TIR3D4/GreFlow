# Operations and recovery

## Files

| Path | Purpose |
|---|---|
| /etc/greflow/config.json | Desired role, endpoints, private /30, MTU, mappings |
| /var/lib/greflow/state.json | Applied config and owned global sysctl writes |
| /var/lib/greflow/backups/TIMESTAMP/ | Previous state, config and diagnostic firewall snapshot |
| /var/log/greflow/events.jsonl | Structured lifecycle events |
| /etc/systemd/system/greflow.service | Reboot reconstruction |
| /run/greflow.lock | Process serialization |

`GREFLOW_ROOT` relocates disk files for integration tests; it does **not** isolate the kernel. Use it only inside isolated network namespaces. It disables systemd persistence. Normal deployments must not set it.

## Backups and rollback

Before host mutations, GreFlow writes its state/config backup and runs `iptables-save -c` for a diagnostic snapshot. The firewall snapshot is not used as a restore script. Only owned rules/interfaces are rebuilt on failure. Recovery errors are included in the error message.

```bash
sudo ls /var/lib/greflow/backups
sudo greflow rollback /var/lib/greflow/backups/TIMESTAMP
sudo greflow doctor
```

A backup with no previous configuration cannot be replayed; use `down`. Endpoint, role, network and interface changes require uninstall/setup; port and MTU changes can be applied in place. All applies recreate the interface, reset counters and clear user verification. Inspect and configure both servers for MTU changes.

Root command failures trigger best-effort rollback. Sudden power loss/SIGKILL is not a fully atomic host transaction: original global sysctl values are persisted before mutation, but a crash may leave partial rules. On recovery, inspect state/backups, run `repair`, then perform an application test. Crash-resumable journaling is on the roadmap.

GreFlow modifies entry `net.ipv4.ip_forward` and `net.ipv4.conf.all.rp_filter` when required. Interface rp_filter is set to zero; deleting the owned interface removes its settings. GreFlow does not blindly tune connection limits or timeouts. Global forwarding/rp_filter affects host routing; review coexistence with your deployment.

## Service state

Interactive setup applies the configuration and enables the service. The oneshot service may initially show inactive until `systemctl start greflow.service` or the next boot; health displays this separately from enabled persistence and actual link/rules. `restart` calls systemd without holding the GreFlow lock, avoiding deadlock with ExecStart/ExecStop.

The unit waits for network-online.target, but this target's behavior depends on the distribution's wait-online service. If your addresses arrive later, enable/configure the appropriate wait-online service and retry `systemctl restart greflow.service`.

`repair` recreates rules and rewrites/enables the systemd unit. A firewall reload may later reorder/remove rules; always verify actual client traffic. Failed unit enable is reported explicitly and does not claim persistence.

## Monitoring and counters

`status` is a snapshot; `test` and `doctor` actively probe. User verification is a manual statement, not a cryptographic or automatic client test. Peer ICMP may fail when remote policies block ping even if some application traffic works. TCP connectivity is not a protocol handshake.

RX/TX are interface counters since interface creation. NAT chains count initial conntrack packets, not total traffic. FORWARD rules count forwarded packets/bytes. Optional `conntrack` counts kernel flow entries for configured destination ports. `ss -s` describes local sockets rather than transit connections.

Audit logs and backups are retained during uninstall for recovery. v0.1 does not rotate them automatically; monitor disk space and apply a retention policy appropriate to your host. Do not remove the active state file while the tunnel is running: originals are needed for safe sysctl restoration.

## Upgrade

The installer backs up an existing binary before replacement. Published version assets are immutable by convention; CI leaves an existing v0.1.1 release intact. Set `GREFLOW_VERSION=vX.Y.Z` for a future published version. Restart only after reviewing release notes and validating the configuration. No telemetry or remote credentials are collected.
