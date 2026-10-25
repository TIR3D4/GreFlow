#!/usr/bin/env bash
set -euo pipefail
[[ $EUID == 0 ]] || { echo 'Run with sudo.' >&2; exit 1; }
here=$(cd -- "$(dirname -- "$0")" && pwd)
id greflow-console >/dev/null 2>&1 || useradd --system --home-dir /var/lib/greflow-console --shell /usr/sbin/nologin greflow-console
install -d -m 0700 -o greflow-console -g greflow-console /var/lib/greflow-console /etc/greflow-console
install -d -m 0755 /opt/greflow-console
install -d -m 0755 /opt/greflow-console/helpers
install -m 0644 "$here/helpers/manifest.json" "$here/helpers/greflow-agent-linux-amd64" "$here/helpers/greflow-agent-linux-arm64" /opt/greflow-console/helpers/
if [[ -f /opt/greflow-console/greflow-console ]]; then
    cp -p /opt/greflow-console/greflow-console /opt/greflow-console/greflow-console.previous
fi
install -m 0755 "$here/greflow-console" /opt/greflow-console/greflow-console.new
mv /opt/greflow-console/greflow-console.new /opt/greflow-console/greflow-console
if [[ ! -f /etc/greflow-console/environment ]]; then
    printf '%s\n' 'GREFLOW_ORIGIN=http://127.0.0.1:8787' > /etc/greflow-console/environment
    chown greflow-console:greflow-console /etc/greflow-console/environment
    chmod 0600 /etc/greflow-console/environment
fi
install -m 0644 "$here/greflow-console.service" /etc/systemd/system/greflow-console.service
systemctl daemon-reload
echo 'Installed without starting services or touching tunnels.'
echo 'Set GREFLOW_ORIGIN for your HTTPS reverse proxy, then: systemctl enable --now greflow-console'
echo 'Open the panel and choose your password on first run. No setup code is needed.'
