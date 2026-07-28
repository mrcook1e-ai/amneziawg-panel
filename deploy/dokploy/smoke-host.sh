#!/usr/bin/env bash
# Isolated host-side smoke test. It deliberately accepts only the development
# host and creates resources whose names include the QA image timestamp.
set -euo pipefail

readonly EXPECTED_HOST='root@144.31.136.132'
readonly DEFAULT_SNIPPET='[Interface]
H1 = 500000000-600000000
H2 = 1500000000-1600000000
H3 = 2600000000-2700000000
H4 = 3700000000-3850000000
S1 = 43
S2 = 65
S3 = 35
S4 = 28
Jc = 5
Jmin = 362
Jmax = 943'

usage() {
  printf 'usage: %s root@144.31.136.132 amneziawg-panel:qa-<timestamp> evidence-dir [--invalid-config-only]\n' "$0" >&2
  exit 64
}

die() {
  printf 'smoke-host: %s\n' "$*" >&2
  exit 1
}

require() {
  command -v "$1" >/dev/null 2>&1 || die "required command is unavailable: $1"
}

[[ $# -eq 3 || $# -eq 4 ]] || usage
remote=$1
image=$2
evidence_dir=$3
mode=happy
if [[ $# -eq 4 ]]; then
  [[ $4 == '--invalid-config-only' ]] || usage
  mode=invalid
fi

[[ $remote == "$EXPECTED_HOST" ]] || die "refusing non-development host: $remote"
if [[ ! $image =~ ^[a-z0-9][a-z0-9._/-]*:qa-([0-9]{8}T[0-9]{6}Z)$ ]]; then
  die "image must be a timestamped qa tag: $image"
fi
stamp=${BASH_REMATCH[1]}

require ssh
require tar
require jq
require curl
require sha256sum
require openssl
mkdir -p "$evidence_dir"
evidence_dir=$(cd "$evidence_dir" && pwd)

repo_root=$(cd "$(dirname "$0")/../.." && pwd)
template="$repo_root/deploy/dokploy/docker-compose.yml"
[[ -f $template ]] || die "Dokploy template not found: $template"

stamp_lc=$(echo "$stamp" | tr '[:upper:]' '[:lower:]')
happy_project="amneziawg-panel-qa-$stamp_lc"
invalid_project="amneziawg-panel-invalid-qa-$stamp_lc"
qa_network="amneziawg-qa-net-$stamp_lc"
if [[ $mode == happy ]]; then
  project=$happy_project
else
  project=$invalid_project
fi
volume="${project}-state"
remote_base="/tmp/amneziawg-panel-qa-$stamp"
local_stage=$(mktemp -d)

cleanup_local() {
  rm -rf "$local_stage"
}
trap cleanup_local EXIT

save_image() {
  if command -v docker >/dev/null 2>&1 && docker image inspect "$image" >/dev/null 2>&1; then
    docker save "$image"
    return
  fi
  if command -v podman >/dev/null 2>&1 && podman image exists "$image"; then
    podman save --format docker-archive "$image"
    return
  fi
  die "the local QA image is not available through docker or podman: $image"
}

if ! ssh "$remote" "docker image inspect '$image' >/dev/null 2>&1"; then
  save_image | ssh "$remote" 'docker load'
  ssh "$remote" "docker tag 'localhost/$image' '$image' 2>/dev/null || true"
fi

stage_project="$local_stage/$mode"
mkdir -p "$stage_project/evidence"
cp "$template" "$stage_project/compose.yml"
cat >"$stage_project/compose.qa.yml" <<EOF
services:
  panel:
    pull_policy: never
    ports:
      - "127.0.0.1::51821/tcp"
volumes:
  amnezia-state:
    name: $volume
networks:
  dokploy-network:
    external: true
    name: $qa_network
EOF
if [[ $mode == invalid ]]; then
  cat >"$stage_project/compose.qa.yml" <<EOF
services:
  panel:
    pull_policy: never
    restart: "no"
    ports: !reset []
volumes:
  amnezia-state:
    name: $volume
networks:
  dokploy-network:
    external: true
    name: $qa_network
EOF
fi

password="qa-password-${stamp}-$(openssl rand -hex 16)"
cat >"$stage_project/.env" <<EOF
PANEL_IMAGE=$image
WG_HOST=144.31.136.132
PASSWORD=$password
WG_PORT_RANGE_START=51860
WG_PORT_RANGE_END=51899
WG_INTERFACE=awg0
WG_PATH=/etc/amnezia/amneziawg
WG_DEFAULT_ADDRESS=10.8.0.x
WG_DEFAULT_DNS=1.1.1.1
WG_ALLOWED_IPS=0.0.0.0/0
WG_MTU=0
WG_PERSISTENT_KEEPALIVE=0
WG_EGRESS_IFACE=
LOG_FORMAT=json
LOG_LEVEL=info
PORT=51821
WEBUI_HOST=0.0.0.0
EOF
chmod 600 "$stage_project/.env"
printf '%s\n' "$DEFAULT_SNIPPET" >"$stage_project/default-obfuscation.conf"
chmod 600 "$stage_project/default-obfuscation.conf"

cat >"$stage_project/remote-run.sh" <<'REMOTE'
#!/usr/bin/env bash
set -euo pipefail

mode=$1
base=$2
project=$3
volume=$4
stamp=$5
dir="$base/$mode"
evidence="$dir/evidence"
stamp_lc=$(echo "$stamp" | tr '[:upper:]' '[:lower:]')
happy_project="amneziawg-panel-qa-$stamp_lc"
happy_dir="$base/happy"
qa_network="amneziawg-qa-net-$stamp_lc"

fail() {
  printf 'remote smoke %s: %s\n' "$mode" "$*" >&2
  exit 1
}

compose() {
  docker compose --project-name "$project" --env-file "$dir/.env" \
    --file "$dir/compose.yml" --file "$dir/compose.qa.yml" "$@"
}

happy_compose() {
  docker compose --project-name "$happy_project" --env-file "$happy_dir/.env" \
    --file "$happy_dir/compose.yml" --file "$happy_dir/compose.qa.yml" "$@"
}

checksum() {
  local name=$1 path=$2
  printf '%s  %s\n' "$(sha256sum "$path" | awk '{print $1}')" "$name"
}

capture_prerequisites() {
  docker version >"$evidence/docker-version.txt"
  docker compose version >"$evidence/docker-compose-version.txt"
  test -c /dev/net/tun || fail '/dev/net/tun is not a character device'
  sysctl -n net.ipv4.ip_forward >"$evidence/ip-forward.txt"
  [[ $(<"$evidence/ip-forward.txt") == 1 ]] || fail 'net.ipv4.ip_forward is not enabled'
  df >"$evidence/df.txt"
  ss -Hlun >"$evidence/ss-hlun.txt"
  command -v curl >/dev/null || fail 'curl is unavailable'
  command -v jq >/dev/null || fail 'jq is unavailable'
}

range_ports() {
  local start end
  start=$(awk -F= '$1 == "WG_PORT_RANGE_START" { print $2; exit }' "$dir/.env")
  end=$(awk -F= '$1 == "WG_PORT_RANGE_END" { print $2; exit }' "$dir/.env")
  awk -v s="$start" -v e="$end" '{ for (i = 1; i <= NF; i++) { p = $i; sub(/^.*:/, "", p); if (p ~ /^[0-9]+$/ && p >= s && p <= e) print p } }' \
    "$evidence/ss-hlun.txt" | sort -nu
}

assert_udp_range_available() {
  local start end
  start=$(awk -F= '$1 == "WG_PORT_RANGE_START" { print $2; exit }' "$dir/.env")
  end=$(awk -F= '$1 == "WG_PORT_RANGE_END" { print $2; exit }' "$dir/.env")
  mapfile -t occupied < <(range_ports)
  if [[ $mode == happy ]]; then
    ((${#occupied[@]} == 0)) || fail "UDP port range $start-$end is occupied"
    return
  fi

  # The required invalid-config run happens after the happy project. Its full
  # UDP range is already owned by that explicitly named QA container; no other
  # listener is accepted.
  local valid_container
  valid_container=$(happy_compose ps -q panel)
  [[ -n $valid_container ]] || fail 'happy QA panel is not running'
  for port in "${occupied[@]}"; do
    docker port "$valid_container" "$port/udp" | grep -q . || fail "unexpected UDP listener on $port"
  done
}

ensure_network() {
	if ! docker network inspect "$qa_network" >/dev/null 2>&1; then
		docker network create "$qa_network" >/dev/null
	fi
}

wait_for_health() {
  local address='' attempt
  for attempt in $(seq 1 30); do
    address=$(compose port panel 51821 2>/dev/null | awk 'NR == 1 { print; exit }')
    address=$(echo "$address" | sed 's/^0\.0\.0\.0:/127.0.0.1:/; s/^:::/127.0.0.1:/')
    if [[ -n $address ]] && curl --fail-with-body --silent --show-error "http://$address/healthz" >"$evidence/healthz.txt" 2>/dev/null; then
      printf '%s' "$address"
      return
    fi
    sleep 1
  done
  fail 'healthz did not become ready'
}

assert_no_secret_in_logs() {
  local logs=$1 password=$2 token=$3 config=$4 private_key psk
  private_key=$(awk -F ' = ' '$1 == "PrivateKey" { print $2; exit }' "$config")
  psk=$(awk -F ' = ' '$1 == "PresharedKey" { print $2; exit }' "$config")
  for secret in "$password" "$token" "$private_key" "$psk"; do
    [[ -z $secret ]] || ! grep -Fq -- "$secret" "$logs" || fail 'a sentinel secret appeared in compose logs'
  done
}

write_manifest() {
  local destination=$1 config=$2 state_dir
  state_dir=$(docker volume inspect "$volume" --format '{{ .Mountpoint }}')
  {
    checksum state.json "$state_dir/state.json"
    checksum awg0.conf "$state_dir/awg0.conf"
    checksum client.conf "$config"
  } >"$destination"
}

run_happy() {
  local address api cookie response subscriber_id cabinet_url token device_id state_dir password
  password=$(awk -F= '$1 == "PASSWORD" { print substr($0, length($1) + 2); exit }' "$dir/.env")
  ensure_network
  compose config --quiet
  compose up -d
  address=$(wait_for_health)
  echo "DEBUG address=$address"
  api="http://$address"
  cookie="$dir/cookies.txt"
  response="$dir/response.json"

  echo "DEBUG calling session"
  curl --fail-with-body --show-error --cookie-jar "$cookie" \
    --header 'Content-Type: application/json' \
    --data "$(jq -cn --arg password "$password" '{password: $password}')" \
    "$api/api/session" >"$response"
  echo "DEBUG session response:"
  cat "$response"
  jq -e '.success == true' "$response" >/dev/null

  echo "DEBUG calling subscribers"
  curl --fail-with-body --show-error --cookie "$cookie" \
    --header 'Content-Type: application/json' \
    --data "$(jq -cn --arg name "qa-subscriber-$stamp" '{name: $name, billingRole: "trusted"}')" \
    "$api/api/subscribers/" >"$response"
  echo "DEBUG subscribers response:"
  cat "$response"
  subscriber_id=$(jq -er '.id' "$response")
  cabinet_url=$(jq -er '.url' "$response")
  token=${cabinet_url##*/cabinet/}
  [[ -n $token && $token != "$cabinet_url" ]] || fail 'subscriber response did not include a cabinet token'

  echo "DEBUG calling devices token=$token"
  curl --fail-with-body --show-error \
    --header 'Content-Type: application/json' \
    --data "$(jq -n --rawfile snippet "$dir/default-obfuscation.conf" --arg device "qa-device-$stamp" '{snippet: $snippet, deviceName: $device}')" \
    "$api/api/cabinet/$token/devices" >"$response"
  echo "DEBUG devices response:"
  cat "$response"
  device_id=$(jq -er '.deviceId' "$response")
  jq -er '.conf' "$response" >"$evidence/client.conf"
  chmod 600 "$evidence/client.conf"
  jq -e --arg id "$device_id" '.deviceId == $id and (.conf | type == "string" and length > 0)' "$response" >/dev/null

  curl --fail-with-body --silent --show-error "$api/healthz" >"$evidence/healthz-after-api.txt"
  curl --fail-with-body --silent --show-error "$api/api/cabinet/$token" >"$response"
  jq -e --arg id "$device_id" '.devices | length == 1 and .[0].id == $id' "$response" >/dev/null
  compose logs --no-color --no-log-prefix >"$evidence/happy-api.jsonl"
  jq -e 'select(.msg == "http request" and .route == "/api/subscribers/" and .status == 200)' "$evidence/happy-api.jsonl" >/dev/null
  jq -e 'select(.msg == "http request" and .route == "/api/cabinet/{token}/devices" and .status == 200)' "$evidence/happy-api.jsonl" >/dev/null
  assert_no_secret_in_logs "$evidence/happy-api.jsonl" "$password" "$token" "$evidence/client.conf"

  write_manifest "$evidence/checksums-before-recreate.txt" "$evidence/client.conf"
  compose up -d --force-recreate
  address=$(wait_for_health)
  api="http://$address"

  curl --fail-with-body --silent --show-error --cookie-jar "$cookie" \
    --header 'Content-Type: application/json' \
    --data "$(jq -cn --arg password "$password" '{password: $password}')" \
    "$api/api/session" >"$response"
  jq -e '.success == true' "$response" >/dev/null
  curl --fail-with-body --silent --show-error --cookie "$cookie" "$api/api/subscribers/$subscriber_id" >"$response"
  jq -e --arg id "$subscriber_id" --arg device "$device_id" '.id == $id and (.devices | length == 1 and .[0].id == $device)' "$response" >/dev/null
  curl --fail-with-body --silent --show-error "$api/api/cabinet/$token/devices/$device_id/configuration" >"$dir/client-after.conf"
  cmp -s "$evidence/client.conf" "$dir/client-after.conf" || fail 'client configuration changed after force recreate'
  write_manifest "$evidence/checksums-after-recreate.txt" "$dir/client-after.conf"
  cmp -s "$evidence/checksums-before-recreate.txt" "$evidence/checksums-after-recreate.txt" || fail 'state/config/key checksums changed after force recreate'

  compose logs --no-color --no-log-prefix >"$evidence/happy-pre-client.jsonl"
  jq -e 'select(.component == "manager" and .msg == "manager ready")' "$evidence/happy-pre-client.jsonl" >/dev/null
  assert_no_secret_in_logs "$evidence/happy-pre-client.jsonl" "$password" "$token" "$evidence/client.conf"
  jq -n --arg mode happy --arg subscriber_id "$subscriber_id" --arg device_id "$device_id" \
    '{mode: $mode, healthz: 200, session: "ok", subscriber_id: $subscriber_id, device_id: $device_id, persistence: "verified", secrets_logged: false}' \
    >"$evidence/happy-summary.json"
  rm -f "$cookie" "$response" "$dir/client-after.conf"
}

run_invalid() {
  local container exit_code before after
  ensure_network
  compose config --quiet
  before=$(ip -o link show | awk -F ': ' '$2 ~ /^awg[0-9]+$/ { print $2 }' | sort)
  compose up -d
  container=$(compose ps -q panel)
  [[ -n $container ]] || fail 'invalid-config container was not created'
  for _ in $(seq 1 20); do
    exit_code=$(docker inspect --format '{{ .State.ExitCode }} {{ .State.Running }}' "$container")
    [[ $exit_code != '0 true' ]] && break
    sleep 1
  done
  [[ $exit_code == '1 false' ]] || fail "invalid-config container did not fail as expected: $exit_code"
  compose logs --no-color --no-log-prefix >"$evidence/invalid-config.jsonl"
  jq -e 'select(.component == "config" and .msg == "network configuration invalid" and .level == "ERROR")' "$evidence/invalid-config.jsonl" >/dev/null
  after=$(ip -o link show | awk -F ': ' '$2 ~ /^awg[0-9]+$/ { print $2 }' | sort)
  [[ $before == "$after" && -z $after ]] || fail 'invalid config created a host awg interface'
  curl --fail-with-body --silent --show-error --retry 3 http://"$(happy_compose port panel 51821 | awk 'NR == 1 { print; exit }')"/healthz >"$evidence/happy-health-after-invalid.txt"
  jq -n --arg mode invalid --arg invalid_host 'https://bad.invalid:51820' \
    '{mode: $mode, invalid_wg_host: $invalid_host, exit_code: 1, component: "config", awg_interfaces_created: 0, happy_project_healthy: true}' \
    >"$evidence/invalid-summary.json"
}

capture_prerequisites
assert_udp_range_available
if [[ $mode == happy ]]; then
  run_happy
else
  run_invalid
fi
REMOTE
chmod 700 "$stage_project/remote-run.sh"

tar -C "$local_stage" -czf - "$mode" | ssh "$remote" "umask 077; mkdir -p '$remote_base'; tar -xzf - -C '$remote_base'"

set +e
if [[ $mode == happy ]]; then
  ssh "$remote" "bash '$remote_base/$mode/remote-run.sh' '$mode' '$remote_base' '$project' '$volume' '$stamp'"
else
  ssh "$remote" "sed -i 's#^WG_HOST=.*#WG_HOST=https://bad.invalid:51820#' '$remote_base/$mode/.env'; bash '$remote_base/$mode/remote-run.sh' '$mode' '$remote_base' '$project' '$volume' '$stamp'"
fi
remote_status=$?
set -e

set +e
ssh "$remote" "tar -C '$remote_base/$mode/evidence' -czf - ." | tar -xzf - -C "$evidence_dir"
copy_status=$?
set -e
[[ $copy_status -eq 0 ]] || die 'could not copy redacted remote evidence'
chmod 600 "$evidence_dir/client.conf" 2>/dev/null || true

if [[ $mode == invalid ]]; then
  ssh "$remote" "set -e; docker compose --project-name '$happy_project' --env-file '$remote_base/happy/.env' --file '$remote_base/happy/compose.yml' --file '$remote_base/happy/compose.qa.yml' stop; docker compose --project-name '$happy_project' --env-file '$remote_base/happy/.env' --file '$remote_base/happy/compose.yml' --file '$remote_base/happy/compose.qa.yml' logs --no-color --no-log-prefix > '$remote_base/happy/evidence/happy-final.jsonl'"
  ssh "$remote" "tar -C '$remote_base/happy/evidence' -czf - happy-final.jsonl" | tar -xzf - -C "$evidence_dir"
  # Stop only the two explicitly named QA projects and remove only their
  # explicitly named state volumes and timestamped QA network after evidence copy.
  # External networks are not removed by `compose down`; delete qa_network explicitly.
  ssh "$remote" "set +e; docker compose --project-name '$invalid_project' --env-file '$remote_base/invalid/.env' --file '$remote_base/invalid/compose.yml' --file '$remote_base/invalid/compose.qa.yml' down --volumes --remove-orphans; docker compose --project-name '$happy_project' --env-file '$remote_base/happy/.env' --file '$remote_base/happy/compose.yml' --file '$remote_base/happy/compose.qa.yml' down --volumes --remove-orphans; docker volume rm -f '$volume' '${happy_project}-state'; docker network rm -f 'amneziawg-qa-net-$stamp_lc' 2>/dev/null; rm -rf '$remote_base'; exit 0"
  jq -e 'select(.component == "lifecycle" and .msg == "signal received")' "$evidence_dir/happy-final.jsonl" >/dev/null
  jq -e 'select(.component == "lifecycle" and .msg == "service stopped")' "$evidence_dir/happy-final.jsonl" >/dev/null
fi

if [[ $remote_status -ne 0 && $mode == happy ]]; then
  ssh "$remote" "set +e; docker compose --project-name '$happy_project' --env-file '$remote_base/happy/.env' --file '$remote_base/happy/compose.yml' --file '$remote_base/happy/compose.qa.yml' down --volumes --remove-orphans; docker volume rm -f '${happy_project}-state'; docker network rm -f 'amneziawg-qa-net-$stamp_lc' 2>/dev/null; rm -rf '$remote_base'; exit 0"
fi

[[ $remote_status -eq 0 ]] || die "remote $mode QA failed; redacted evidence was copied before cleanup"
printf 'smoke-host %s succeeded; evidence: %s\n' "$mode" "$evidence_dir"
