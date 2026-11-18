#!/bin/sh
# shint platform test for openbsd/arm64 - made by test/platforms/platforms.py from template.sh:
# edit the template, not this file, then run `make platform-tests`.
#
# Run it on a openbsd/arm64 machine with that release's binary, in a shell:
#   sh openbsd-arm64.sh ./shint.openbsd.arm64
# It writes results/openbsd-arm64.txt next to itself. Send that file back:
# https://github.com/dmartsapp/shint/issues/new
#
# What it does: starts shint's own TCP, UDP and HTTP listeners on 127.0.0.1, checks
# them with telnet, nmap, udp, wol and web, runs the commands that read this machine
# (ip, ip route, ip arp, ss, dns and rdns from the hosts file, cidr, ping 127.0.0.1),
# and - when example.com answers - a few checks over the internet (dns, web, tls,
# ntp). About a minute. It changes nothing on the machine and needs no root; run it
# as a normal user.
#
# The results file holds which release was tested (its version and the binary's
# SHA-256), what the machine is (system and version - never its host name or
# addresses), how many checks passed, and for each failure its exit status and the
# error lines shint printed - never a table of sockets, routes or neighbours.

PLATFORM=openbsd/arm64
SLUG=openbsd-arm64
ASSET=shint.openbsd.arm64
EXPECT_SS=no      # ss: yes = works; no = says "not supported yet"; absent = not in this binary
EXPECT_ROUTE=yes   # ip route and ip arp: yes = work; no = say "not supported yet"
EXPECT_PING=yes    # ping 127.0.0.1: yes = must answer; sysctl = a permission error is a skip

set -u

here=$(cd "$(dirname "$0")" && pwd)
results_dir="$here/results"
out="$results_dir/$SLUG.txt"

# The binary: the argument, or the release's file name here, or ./shint, or shint on PATH.
bin=${1:-}
if [ -z "$bin" ]; then
  for c in "./$ASSET" "$here/$ASSET" ./shint; do
    if [ -f "$c" ]; then bin=$c; break; fi
  done
fi
if [ -z "$bin" ] && command -v shint >/dev/null 2>&1; then bin=$(command -v shint); fi
if [ -z "$bin" ] || [ ! -f "$bin" ]; then
  echo "usage: sh $(basename "$0") path/to/$ASSET" >&2
  echo "  (the $PLATFORM binary from https://github.com/dmartsapp/shint/releases)" >&2
  exit 2
fi
if [ ! -x "$bin" ]; then
  echo "$bin is not executable: chmod +x $bin" >&2
  exit 2
fi
case $bin in */*) ;; *) bin=./$bin ;; esac

# Which release, and for which platform: "v4.4.0/<commit>/<built> (go1.27.4 linux/amd64)",
# or "4.4.0 (go1.27.4 linux/amd64)" for a build made without the release's stamp.
version=$("$bin" --version 2>&1) || true
binplat=$(printf '%s\n' "$version" | sed -n 's/.*(go[^ ]* \([^)]*\)).*/\1/p')
goversion=$(printf '%s\n' "$version" | sed -n 's/.*(\(go[^ ]*\) .*/\1/p')
stamp=${version%% (*}
release=${stamp%%/*}
commit=""
built=""
case $stamp in
  */*/*) rest=${stamp#*/}; commit=${rest%%/*}; built=${rest#*/} ;;
esac
if [ "$binplat" != "$PLATFORM" ]; then
  echo "$bin is not a $PLATFORM build of shint (--version says: $version)." >&2
  echo "Use the script named after its platform, or the $ASSET binary." >&2
  exit 2
fi

mkdir -p "$results_dir" || exit 2
work="$results_dir/.work.$$"
mkdir "$work" || exit 2
pids=""
cleanup() {
  for p in $pids; do kill "$p" 2>/dev/null; done
  rm -rf "$work"
}
trap cleanup 0
trap 'exit 130' INT TERM

sha256() {
  for tool in "sha256sum" "shasum -a 256" "sha256 -q" "digest -a sha256" "cksum -a sha256" "csum -h SHA256" "openssl dgst -sha256"; do
    # shellcheck disable=SC2086
    set -- $tool
    command -v "$1" >/dev/null 2>&1 || continue
    sum=$("$@" "$bin" 2>/dev/null | tr 'A-F' 'a-f' | sed -n 's/.*\([0-9a-f]\{64\}\).*/\1/p' | head -n 1)
    if [ -n "$sum" ]; then echo "$sum"; return; fi
  done
  echo unknown
}

os_name() {
  s="$(uname -s 2>/dev/null) $(uname -r 2>/dev/null)"
  case $PLATFORM in
    darwin/*)  s="$s, macOS $(sw_vers -productVersion 2>/dev/null)" ;;
    android/*) s="$s, Android $(getprop ro.build.version.release 2>/dev/null)" ;;
    aix/*)     s="$s, AIX $(oslevel 2>/dev/null)" ;;
    solaris/* | illumos/*) s="$s, $(uname -v 2>/dev/null)" ;;
    linux/*)   if [ -r /etc/os-release ]; then s="$s, $(sed -n 's/^PRETTY_NAME="*\([^"]*\)"*$/\1/p' /etc/os-release)"; fi ;;
    *bsd/*)    ;;
  esac
  echo "$s"
}

passed=0
failed=0
skipped=0
: > "$work/lines"

record() { # STATUS NAME [DETAIL]
  printf '%-4s  %s\n' "$1" "$2" | tee -a "$work/lines"
  if [ -n "${3:-}" ]; then printf '%s\n' "$3" | sed -e '/^$/d' -e 's/^/      /' | tee -a "$work/lines"; fi
  case $1 in
    PASS) passed=$((passed + 1)) ;;
    FAIL) failed=$((failed + 1)) ;;
    SKIP) skipped=$((skipped + 1)) ;;
  esac
}

run() { # shint's arguments: sets $code, output in $work/out
  "$bin" "$@" > "$work/out" 2>&1
  code=$?
}

# A failure's detail: shint's error lines only (never the tables), without the date.
errors() {
  grep -iE 'error|panic|fatal|denied|not permitted' "$work/out" | head -n 5 \
    | sed 's/^[A-Z][a-z][a-z] [A-Z][a-z][a-z] [ 0-9]*[0-9] [0-9:]* [^ ]* [0-9]*: //' | cut -c1-300
}

check() { # NAME EXIT PATTERN shint-arguments...
  name=$1; want=$2; pattern=$3; shift 3
  run "$@"
  if [ "$code" = "$want" ] && grep -E -q -- "$pattern" "$work/out"; then
    record PASS "$name"
  else
    record FAIL "$name" "exit=$code (wanted $want), wanted a line matching: $pattern
$(errors)"
  fi
}

# Like check, but a failure is a skip: for what depends on the network around the
# machine rather than on shint (a firewall blocking NTP, say).
soft() { # NAME REASON EXIT PATTERN shint-arguments...
  name=$1; reason=$2; want=$3; pattern=$4; shift 4
  run "$@"
  if [ "$code" = "$want" ] && grep -E -q -- "$pattern" "$work/out"; then
    record PASS "$name"
  else
    record SKIP "$name" "$reason (exit=$code)
$(errors)"
  fi
}

listen() { # NAME LOGFILE shint-arguments...: starts a listener, waits for it
  name=$1; log=$2; shift 2
  "$bin" "$@" > "$log" 2>&1 &
  pid=$!
  pids="$pids $pid"
  n=0
  while [ $n -lt 10 ]; do
    if grep -q 'listening address=' "$log"; then record PASS "$name"; return 0; fi
    if ! kill -0 "$pid" 2>/dev/null; then break; fi
    sleep 1
    n=$((n + 1))
  done
  cp "$log" "$work/out"
  record FAIL "$name" "it did not start listening
$(errors)"
  return 1
}

echo "shint $release on $PLATFORM - results go to $out"
echo

# --- the binary itself ---
record PASS "version: --version names $PLATFORM ($release)"
check "help: lists the commands" 0 "Available Commands" --help

# --- listeners on 127.0.0.1, and the checks against them ---
base=$((20000 + $$ % 20000))
tcp_port=$base
udp_port=$((base + 1))
http_port=$((base + 2))
wol_port=$((base + 3))
closed_port=$((base + 4))

listen "listen tcp: starts on 127.0.0.1" "$work/listen-tcp" listen tcp "$tcp_port" --bind 127.0.0.1 --timeout 0
tcp_pid=$pid
listen "listen udp: starts on 127.0.0.1, echoing" "$work/listen-udp" listen udp "$udp_port" --bind 127.0.0.1 --echo
listen "listen http: starts on 127.0.0.1" "$work/listen-http" listen http "$http_port" --bind 127.0.0.1
listen "listen udp: a second one, for wol" "$work/listen-wol" listen udp "$wol_port" --bind 127.0.0.1

check "telnet: connects to the local TCP listener" 0 "connect ok host=127.0.0.1 port=$tcp_port" telnet 127.0.0.1 "$tcp_port"
check "telnet: a closed port is a failed check (exit 1)" 1 "ERROR" telnet 127.0.0.1 "$closed_port"
check "telnet --interface 127.0.0.1: sends from that address" 0 "source interface=127.0.0.1" telnet 127.0.0.1 "$tcp_port" --interface 127.0.0.1
check "--interface: an unknown name fails and lists the real ones" 1 "no such interface nosuch0 \(available: " telnet 127.0.0.1 "$tcp_port" --interface nosuch0
check "nmap -p: one open port and one closed" 0 "scan complete ports_scanned=2 open=1" nmap 127.0.0.1 -p "$tcp_port,$closed_port"
check "nmap --json: the open port in stats" 0 "\"port\": $tcp_port" nmap 127.0.0.1 -p "$tcp_port" --json
check "udp: the echoing listener answers" 0 "probe open host=127.0.0.1 port=$udp_port" udp 127.0.0.1 "$udp_port" --data shint
check "wol: sends a magic packet" 0 "magic packet sent" wol 02:00:00:00:00:01 --broadcast 127.0.0.1 --port "$wol_port"
sleep 1
if grep -q "packet received .*bytes=102" "$work/listen-wol"; then
  record PASS "wol: the listener received the 102-byte packet"
else
  cp "$work/listen-wol" "$work/out"
  record FAIL "wol: the listener received the 102-byte packet" "$(errors)"
fi
check "web: GET from the local HTTP listener" 0 "status=200" web "http://127.0.0.1:$http_port/"
check "web --json: status_code 200" 0 "\"status_code\": 200" web "http://127.0.0.1:$http_port/" --json
check "web --headers-only: the response's headers" 0 "header name=Content-Type" web "http://127.0.0.1:$http_port/" --headers-only

# --- this machine ---
check "ip: lists the interfaces, loopback included" 0 "127\.0\.0\.1" ip
check "cidr: a /24 has 254 hosts" 0 "hosts=254" cidr 192.168.1.0/24
check "dns: localhost from the hosts file" 0 "data=127\.0\.0\.1" dns localhost A
check "rdns: 127.0.0.1 has a name" 0 "names=\[" rdns 127.0.0.1

if [ "$EXPECT_ROUTE" = yes ]; then
  check "ip route: lists the routing table" 0 "OK route destination=" ip route
  check "ip route 127.0.0.1: goes out through loopback" 0 "source=127\.0\.0\.1" ip route 127.0.0.1
  check "ip arp: reads the neighbour table" 0 "OK done" ip arp
else
  check "ip route: says it is not supported yet (exit 1)" 1 "not supported" ip route
  check "ip arp: says it is not supported yet (exit 1)" 1 "not supported" ip arp
fi

case $EXPECT_SS in
  yes)
    check "ss -l: finds the TCP listener and its process" 0 "pid=$tcp_pid( |$)" ss -l --ports "$tcp_port"
    check "ss --json: the listener's process" 0 "\"pid\": $tcp_pid([^0-9]|$)" ss -l --ports "$tcp_port" --json
    check "ss: nothing matching is a failed check (exit 1)" 1 "." ss --ports "$closed_port"
    ;;
  no)
    check "ss: says it is not supported yet (exit 1)" 1 "not supported" ss -l
    ;;
  absent)
    check "ss: not in this binary (unknown command, exit 2)" 2 "unknown command" ss -l
    ;;
esac

run ping 127.0.0.1
if [ "$code" = 0 ] && grep -q "received reply" "$work/out"; then
  record PASS "ping: 127.0.0.1 answers"
elif [ "$EXPECT_PING" = sysctl ] && grep -qiE "permission|not permitted" "$work/out"; then
  record SKIP "ping: 127.0.0.1 answers" "this system does not allow ICMP for normal users (net.ipv4.ping_group_range); run as root, or see the ping page
$(errors)"
else
  record FAIL "ping: 127.0.0.1 answers" "exit=$code (wanted 0)
$(errors)"
fi

# --- over the internet, when there is one ---
run telnet example.com 443 -4
if [ "$code" = 0 ]; then
  internet=yes
  record PASS "telnet: example.com 443 over the internet"
  check "dns: example.com through the system's name server" 0 "type=A .*answers=[1-9]" dns example.com A
  check "web: https://example.com, its certificate trusted" 0 "status=200" web https://example.com -4
  check "tls: example.com's chain verified" 0 "OK verified server=example.com" tls example.com -4
  soft "ntp: pool.ntp.org answers" "no NTP answer - UDP port 123 may be blocked here" 0 "OK response server=pool.ntp.org" ntp pool.ntp.org -4
else
  internet=no
  record SKIP "internet checks" "example.com:443 did not answer, so dns, web, tls and ntp over the internet were not run"
fi

# --- the results file ---
for p in $pids; do kill "$p" 2>/dev/null; done
pids=""
if [ "$failed" = 0 ]; then result=pass; else result=fail; fi
if [ "$(id -u)" = 0 ]; then privileges=root; else privileges="normal user"; fi
{
  echo "# shint platform test: $PLATFORM"
  echo "# Written by test/platforms/$(basename "$0"); run it again to replace this file."
  echo "platform: $PLATFORM"
  echo "release: $release"
  echo "commit: $commit"
  echo "built: $built"
  echo "go: $goversion"
  echo "binary: $(basename "$bin")"
  echo "binary_sha256: $(sha256)"
  echo "tested_at: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  echo "os: $(os_name)"
  echo "privileges: $privileges"
  echo "internet: $internet"
  echo "passed: $passed"
  echo "failed: $failed"
  echo "skipped: $skipped"
  echo "result: $result"
  echo
  cat "$work/lines"
} > "$out"

echo
echo "$passed passed, $failed failed, $skipped skipped - $out"
echo "Please send that file: https://github.com/dmartsapp/shint/issues/new"
if [ "$failed" = 0 ]; then exit 0; else exit 1; fi
