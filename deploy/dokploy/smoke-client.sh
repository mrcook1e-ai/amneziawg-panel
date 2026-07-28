#!/usr/bin/env bash
# Run the generated client config in an ephemeral, isolated network namespace.
set -euo pipefail

usage() {
  printf 'usage: %s /path/to/client.conf amneziawg-panel:qa-<timestamp>\n' "$0" >&2
  exit 64
}

die() {
  printf 'smoke-client: %s\n' "$*" >&2
  exit 1
}

[[ $# -eq 2 ]] || usage
config=$1
image=$2
[[ -f $config ]] || die "configuration not found: $config"
[[ -c /dev/net/tun ]] || die '/dev/net/tun is not a character device'

if command -v docker >/dev/null 2>&1 && docker image inspect "$image" >/dev/null 2>&1; then
  runtime=docker
  mount="$config:/work/client.conf:ro"
elif command -v podman >/dev/null 2>&1 && podman image exists "$image"; then
  runtime=podman
  mount="$config:/work/client.conf:ro,Z"
else
  die "the local QA image is not available through docker or podman: $image"
fi

evidence_dir=$(cd "$(dirname "$config")" && pwd)
client_evidence="$evidence_dir/client-smoke.jsonl"

"$runtime" run --rm --cap-add NET_ADMIN --device /dev/net/tun \
  --volume "$mount" \
  --entrypoint /bin/sh "$image" -ec '
    set -eu
    teardown() { awg-quick down /work/client.conf >/dev/null 2>&1 || true; }
    trap teardown EXIT INT TERM
    awg-quick up /work/client.conf
    awg show > /tmp/awg-before.txt
    grep -q "interface:" /tmp/awg-before.txt
    nslookup api.ipify.org >/tmp/dns.txt
    wget -qO /tmp/egress.txt https://api.ipify.org
    test -s /tmp/egress.txt
    awg show > /tmp/awg-after.txt
    grep -q "interface:" /tmp/awg-after.txt
  '

printf '{"event":"client_smoke","interface":"up-down","dns":"api.ipify.org","egress":"https://api.ipify.org","result":"pass"}\n' >"$client_evidence"
chmod 600 "$client_evidence"
printf 'smoke-client succeeded; evidence: %s\n' "$client_evidence"
