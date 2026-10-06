#!/usr/bin/env bash
# Download only GreFlow release artifacts; verify before replacing the binary.
set -euo pipefail
[[ ${EUID} -eq 0 ]] || { echo 'Run as root or with sudo.' >&2; exit 1; }
[[ $(uname -s) == Linux ]] || { echo 'Linux required.' >&2; exit 1; }
version=${GREFLOW_VERSION:-v0.2.0}
[[ $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid version.' >&2; exit 1; }
case "$(uname -m)" in x86_64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo 'Supported architectures: amd64, arm64' >&2; exit 1 ;; esac
for command in curl sha256sum ip iptables iptables-save sysctl systemctl; do
    command -v "$command" >/dev/null || { echo "Missing $command. On Debian/Ubuntu: apt-get update && apt-get install -y curl ca-certificates iproute2 iptables procps iputils-ping" >&2; exit 1; }
done
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
base="https://github.com/TIR3D4/GreFlow/releases/download/$version"
file="greflow-linux-$arch"
curl --proto '=https' --tlsv1.2 -fL --retry 3 "$base/$file" -o "$work/$file"
curl --proto '=https' --tlsv1.2 -fL --retry 3 "$base/SHA256SUMS" -o "$work/SHA256SUMS"
awk -v f="$file" '$2 == f { print }' "$work/SHA256SUMS" > "$work/checksum"
[[ $(wc -l < "$work/checksum") -eq 1 ]] || { echo 'Missing/ambiguous checksum' >&2; exit 1; }
(cd "$work" && sha256sum -c checksum)
chmod 0755 "$work/$file"
"$work/$file" version
if [[ -f /usr/local/bin/greflow ]]; then
    mkdir -p /var/lib/greflow/backups
    cp -p /usr/local/bin/greflow "/var/lib/greflow/backups/greflow-$(date -u +%Y%m%dT%H%M%S)"
fi
install -m 0755 "$work/$file" /usr/local/bin/.greflow-new
mv -f /usr/local/bin/.greflow-new /usr/local/bin/greflow
if [[ -f /etc/greflow/config.json || -d /etc/greflow/tunnels ]]; then
    echo 'Upgraded; existing configs retained, no tunnel restarted. Run: greflow tunnels'
else
    echo 'Installed. Run: greflow setup'
fi
