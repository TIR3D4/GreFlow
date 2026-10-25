#!/usr/bin/env bash
set -euo pipefail
[[ $EUID == 0 ]] || { echo 'Run with sudo.' >&2; exit 1; }
here=$(cd -- "$(dirname -- "$0")" && pwd)
for tool in ip iptables iptables-save sysctl systemctl ping; do
    command -v "$tool" >/dev/null || { echo "Missing dependency: $tool" >&2; exit 1; }
done
install -m 0755 "$here/greflow-agent" /usr/local/bin/greflow-agent.new
mv /usr/local/bin/greflow-agent.new /usr/local/bin/greflow-agent
# Preserve the existing engine and configuration. Installing the companion is
# never permission to replace an installed CLI or restart any tunnel.
if [[ ! -e /usr/local/bin/greflow ]]; then
    install -m 0755 "$here/greflow" /usr/local/bin/greflow
fi
install -m 0644 "$here/greflow-agent.service" /etc/systemd/system/greflow-agent.service
systemctl daemon-reload
echo 'Helper installed; no GRE/firewall changes and no service started.'
echo 'SSH: grant the chosen trusted management user sudo only for /usr/local/bin/greflow-agent dispatch.'
echo 'Outbound: enroll using the console token, then systemctl enable --now greflow-agent.'
