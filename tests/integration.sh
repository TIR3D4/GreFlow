#!/usr/bin/env bash
# Isolated real-kernel test: no rules or sysctls are changed in the host namespace.
set -euo pipefail
[[ ${EUID} -eq 0 ]] || { echo 'Root required'; exit 1; }
binary=$(realpath "${1:-./dist/greflow-linux-amd64}")
work=$(mktemp -d)
suffix=$$
entry="gf-entry-$suffix"
exitns="gf-exit-$suffix"
client="gf-client-$suffix"
bridge="gfb$suffix"
pids=()
cleanup() {
    for pid in "${pids[@]}"; do kill "$pid" 2>/dev/null || true; done
    for ns in "$entry" "$exitns" "$client"; do ip netns del "$ns" 2>/dev/null || true; done
    ip link del "$bridge" 2>/dev/null || true
    rm -rf "$work"
}
trap cleanup EXIT
ip link add "$bridge" type bridge
ip link set "$bridge" up
for spec in "$entry:1" "$exitns:2" "$client:3"; do
    ns=${spec%:*}; octet=${spec##*:}
    ip netns add "$ns"
    ip link add "gfv${suffix}$octet" type veth peer name eth0 netns "$ns"
    ip link set "gfv${suffix}$octet" master "$bridge"
    ip link set "gfv${suffix}$octet" up
    ip -n "$ns" addr add "192.0.2.$octet/24" dev eth0
    ip -n "$ns" link set eth0 up
    ip -n "$ns" link set lo up
    ip netns exec "$ns" iptables -A INPUT -p tcp --dport 2222 -j DROP
    ip netns exec "$ns" iptables -P INPUT DROP
    ip netns exec "$ns" iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
    ip netns exec "$ns" iptables -P FORWARD DROP
 done
gf() { local ns=$1; shift; ip netns exec "$ns" env GREFLOW_ROOT="$work/$ns" "$binary" "$@"; }
original_forward=$(ip netns exec "$entry" sysctl -n net.ipv4.ip_forward)
original_rpf=$(ip netns exec "$entry" sysctl -n net.ipv4.conf.all.rp_filter)
gf "$entry" setup --role entry --local 192.0.2.1 --remote 192.0.2.2 --no-persist
gf "$exitns" setup --role exit --local 192.0.2.2 --remote 192.0.2.1 --no-persist
gf "$entry" add tcp 443 2020
gf "$entry" add tcp 8443 2020
gf "$exitns" add tcp 2020
gf "$entry" add udp 444 2021
gf "$exitns" add udp 2021
gf "$entry" add udp 12000-12002
gf "$exitns" add udp 12000-12002
cat > "$work/echo.py" <<'PYTHON'
import socket, threading, time
ready=0
lock=threading.Lock()
def echo(proto,port):
 global ready
 s=socket.socket(socket.AF_INET, socket.SOCK_STREAM if proto=='tcp' else socket.SOCK_DGRAM)
 s.bind(('10.77.0.2',port))
 if proto=='tcp': s.listen()
 with lock: ready+=1
 while True:
  if proto=='tcp':
   c,_=s.accept()
   with c: c.sendall(c.recv(1024))
  else:
   data,addr=s.recvfrom(1024);s.sendto(data,addr)
for proto,port in [('tcp',2020),('udp',2021),('udp',12000),('udp',12001),('udp',12002)]:
 threading.Thread(target=echo,args=(proto,port),daemon=True).start()
while ready!=5: time.sleep(.02)
open(__import__('sys').argv[1],'w').write('ready')
threading.Event().wait()
PYTHON
ip netns exec "$exitns" python3 "$work/echo.py" "$work/ready" &
pids+=("$!")
for _ in {1..50}; do [[ -f "$work/ready" ]] && break; sleep 0.1; done
[[ -f "$work/ready" ]]
probe() {
 ip netns exec "$client" python3 - <<'PYTHON'
import socket
for proto,port in [('tcp',443),('tcp',8443),('udp',444),('udp',12000),('udp',12001),('udp',12002)]:
 s=socket.socket(socket.AF_INET,socket.SOCK_STREAM if proto=='tcp' else socket.SOCK_DGRAM)
 s.settimeout(3);s.connect(('192.0.2.1',port));s.sendall(b'GreFlow-data-path')
 assert s.recv(1024)==b'GreFlow-data-path',(proto,port)
 s.close()
PYTHON
}
probe
[[ $(ip netns exec "$entry" iptables -t nat -S GREFLOW_POSTROUTING | grep -c -- '-p tcp') -eq 1 ]]
# Inject one command failure, then let rollback use the real backend.
real_iptables=$(command -v iptables)
mkdir -p "$work/fault-bin"
cat > "$work/fault-bin/iptables" <<'WRAPPER'
#!/usr/bin/env bash
set -euo pipefail
if [[ $* == *'-A GREFLOW_PREROUTING'* && ! -f $GREFLOW_FAULT_FILE ]]; then
    touch "$GREFLOW_FAULT_FILE"
    echo 'Injected integration failure' >&2
    exit 1
fi
exec "$GREFLOW_REAL_IPTABLES" "$@"
WRAPPER
chmod +x "$work/fault-bin/iptables"
if PATH="$work/fault-bin:$PATH" GREFLOW_FAULT_FILE="$work/failed-once" GREFLOW_REAL_IPTABLES="$real_iptables" gf "$entry" add udp 445 2021; then
    echo 'Expected injected failure' >&2
    exit 1
fi
[[ -f "$work/failed-once" ]]
if gf "$entry" list | grep -q '^UDP 445 '; then exit 1; fi
probe
for _ in {1..3}; do gf "$entry" apply; done
[[ $(ip netns exec "$entry" iptables -S FORWARD | grep -c 'greflow:hook') -eq 1 ]]
probe
ip netns exec "$entry" iptables -D FORWARD -m comment --comment greflow:hook -j GREFLOW_FORWARD
gf "$entry" repair
probe
gf "$entry" test
gf "$entry" remove tcp 443
if ip netns exec "$entry" iptables -t nat -S GREFLOW_PREROUTING | grep -q -- '--dport 443'; then exit 1; fi
gf "$entry" add tcp 443 2020
probe
gf "$entry" down
if ip -n "$entry" link show greflow0 >/dev/null 2>&1; then exit 1; fi
gf "$entry" apply
probe
gf "$entry" uninstall --yes
gf "$exitns" uninstall --yes
[[ $(ip netns exec "$entry" sysctl -n net.ipv4.ip_forward) == "$original_forward" ]]
[[ $(ip netns exec "$entry" sysctl -n net.ipv4.conf.all.rp_filter) == "$original_rpf" ]]
for ns in "$entry" "$exitns"; do
    ip netns exec "$ns" iptables -C INPUT -p tcp --dport 2222 -j DROP
    if ip netns exec "$ns" iptables-save | grep -q GREFLOW_; then exit 1; fi
    if ip -n "$ns" link show greflow0 >/dev/null 2>&1; then exit 1; fi
 done
echo 'PASS: GRE, TCP mapping, UDP mapping/range, reapply, repair, down/apply, removal, ownership, shared destinations, failure rollback, sysctl restore'
