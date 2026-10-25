#!/usr/bin/env bash
set -euo pipefail
[[ $EUID == 0 ]] || { echo 'Run with sudo.' >&2; exit 1; }
systemctl disable --now greflow-console.service
rm -f /etc/systemd/system/greflow-console.service /opt/greflow-console/greflow-console
systemctl daemon-reload
echo 'Console removed. Database, keys, agent, remote tunnels and schedules retained.'
