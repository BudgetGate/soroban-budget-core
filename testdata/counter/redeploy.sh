#!/usr/bin/env bash
# Redeploys the counter contract v1 and v2 to testnet and rebuilds the fixtures and snapshots,
# for when a testnet reset has removed the contracts or the fixtures' source account.
#
# Usage: testdata/counter/redeploy.sh [output-dir]
# Writes fixture_v1.json, fixture_v2.json, snapshot_v1.json and snapshot_v2.json to output-dir
# (default: this directory, replacing the committed files). Needs the stellar CLI, the
# wasm32v1-none Rust target and Go. Afterwards, update the contract IDs and the result table
# in README.md, and copy fixture_v1.json to budgetgate-action-test's budget/fixture.json.
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
out="$(mkdir -p "${1:-$here}" && cd "${1:-$here}" && pwd)"
key=budgetgate-verify
network=testnet

cd "$here"

# After a reset the account is gone, so fund it again; a funded account makes friendbot fail, which is fine.
if stellar keys ls | grep -qx "$key"; then
  stellar keys fund "$key" --network "$network" || echo "Funding $key failed; continuing, it may already be funded." >&2
else
  stellar keys generate "$key" --network "$network" --fund
fi

trap 'cp contracts/counter/src/lib_v1.rs contracts/counter/src/lib.rs' EXIT

# Testnet RPC calls sometimes time out; try each network step up to three times.
retry() {
  local attempt
  for attempt in 1 2 3; do
    "$@" && return 0
    echo "Attempt $attempt of '$1 $2 $3' failed." >&2
    [ "$attempt" -lt 3 ] && sleep 15
  done
  return 1
}

deploy() {
  local version="$1"
  cp "contracts/counter/src/lib_$version.rs" contracts/counter/src/lib.rs
  stellar contract build >&2
  local id xdr
  id="$(retry stellar contract deploy --wasm target/wasm32v1-none/release/counter.wasm --source "$key" --network "$network")"
  xdr="$(retry stellar contract invoke --id "$id" --source "$key" --network "$network" --build-only -- bump)"
  printf '[{"name":"bump","fixture_id":"counter_bump","xdr":"%s"}]\n' "$xdr" > "$out/fixture_$version.json"
  echo "$version contract: $id"
}

deploy v1
deploy v2

core="$(mktemp -d)"
(cd "$here/../.." && go build -o "$core/budget-core" .)
echo '{"schema_version":"2.0","functions":[]}' > "$core/empty.json"

"$core/budget-core" --baseline "$core/empty.json" --fixture "$out/fixture_v1.json" --output-json "$out/snapshot_v1.json"

status=0
"$core/budget-core" --baseline "$out/snapshot_v1.json" --fixture "$out/fixture_v2.json" --output-json "$out/snapshot_v2.json" || status=$?
rm -r "$core"
if [ "$status" -ne 1 ]; then
  echo "Expected core to exit 1 (regression) on v2 against v1, got $status." >&2
  exit 1
fi
echo "Wrote fixtures and snapshots to $out." >&2
