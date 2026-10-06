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

## v0.2 named instance operations and upgrade

The default legacy paths/alias/service remain compatible. Upgrade the binary with install.sh; it preserves configs and does not restart tunnels. There is no on-disk migration. A rollback to v0.1 supports only the old default, so remove/stop named services before downgrading the binary.

`greflow tunnels` lists all configurations. Commands accept `--tunnel NAME` before or after the command. Default can be selected by omitting the flag or using `--tunnel default`. Named backups are in `/var/lib/greflow/backups/NAME/TIMESTAMP`; use the matching selector when rolling them back. Lifecycle logs are shared JSONL with a tunnel label.

Uninstall is instance-scoped. Removing default does not delete named config directories; removing a named instance does not delete default. The installed binary stays until no configured siblings remain. Stop/disable a named tunnel persistently with `systemctl disable --now greflow-NAME.service`.

Startup operations wait up to 60 seconds for the host-wide lock; unit timeout remains 120 seconds. Typical configs start quickly, but large rule sets or stalled system commands can time out. No blanket concurrency promise is made for arbitrarily large installations. Configs reserve interfaces, networks and entry public ports even when inactive; uninstall an unused config to release those reservations.

Existing active entries share a saved global sysctl baseline. Keep state files intact. Removing one does not disable forwarding for its peers. Health refreshes interface counters after active probes; packet counters/ICMP/TCP still are not a full Xray handshake. v0.2 never forces the host's global ICMP echo policy to change.
