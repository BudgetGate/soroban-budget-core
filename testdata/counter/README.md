# Counter contract: measurement check

A small Soroban contract used to check that core's measurement moves when a contract's cost moves (`CLAUDE.md` section 6, rule 3): deploy it, change it so its cost should rise, redeploy, and confirm core reports the rise.

| File | What it is |
|---|---|
| `contracts/counter/src/lib_v1.rs` | `bump` reads and writes one persistent key. |
| `contracts/counter/src/lib_v2.rs` | Same, plus three extra persistent writes and three `sha256` host calls. |
| `contracts/counter/src/lib.rs` | The version that gets built. A copy of v1; copy v1 or v2 over it before building. |
| `Cargo.toml`, `Cargo.lock`, `contracts/counter/Cargo.toml` | Rust workspace (`soroban-sdk` 28). |
| `fixture_v1.json`, `fixture_v2.json` | Fixtures with one `bump` call each, built with `stellar contract invoke --build-only` against the deployed v1 and v2 contracts. Unsigned; the source account is a public key only. |
| `snapshot_v1.json`, `snapshot_v2.json` | Snapshots written by core from the two fixtures on 2026-10-27, protocol 29. `TestCounterContractSnapshots` compares them offline. |

Deployed on testnet on 2026-10-26: v1 `CBRHXROBMMVVQAIQWJOSI4LJZFAOAO7BF5XKPMZHJ7NBIL3DSSA2MM4W`, v2 `CB4RIW7CU345POFUEQYLVXOZW52TKVCMCDZDA5N4XMB6LTW7ON3VHNKX`.

## Result

Measured with core from `main` on 2026-10-27 (the same numbers as the first run on 2026-10-26):

| Metric | v1 | v2 |
|---|---|---|
| CPU instructions | 291,573 | 449,005 |
| Read bytes | 0 | 0 |
| Write bytes | 80 | 380 |
| Read entries | 3 | 6 |
| Write entries | 1 | 4 |

Core exits 1 on v2 against the v1 snapshot. v1 against its own snapshot shows 0.00% on every metric.

## Rerun against testnet

Needs the `stellar` CLI (28.1.0 was used) and the `wasm32v1-none` Rust target. From the repository root:

```bash
go build -o budget-core .
echo '{"schema_version":"2.0","functions":[]}' > /tmp/empty.json
./budget-core --baseline /tmp/empty.json --fixture testdata/counter/fixture_v1.json --output-json /tmp/snapshot_v1.json
./budget-core --baseline /tmp/snapshot_v1.json --fixture testdata/counter/fixture_v2.json   # expect exit 1
```

Testnet is reset periodically, which removes the contracts and the fixtures' source account; simulation then fails. Redeploy and rebuild the fixtures from `testdata/counter/`:

```bash
stellar keys generate budgetgate-verify --network testnet --fund   # once per machine

cp contracts/counter/src/lib_v1.rs contracts/counter/src/lib.rs
stellar contract build
V1=$(stellar contract deploy --wasm target/wasm32v1-none/release/counter.wasm --source budgetgate-verify --network testnet)
X1=$(stellar contract invoke --id $V1 --source budgetgate-verify --network testnet --build-only -- bump)
echo "[{\"name\":\"bump\",\"fixture_id\":\"counter_bump\",\"xdr\":\"$X1\"}]" > fixture_v1.json

cp contracts/counter/src/lib_v2.rs contracts/counter/src/lib.rs
stellar contract build
V2=$(stellar contract deploy --wasm target/wasm32v1-none/release/counter.wasm --source budgetgate-verify --network testnet)
X2=$(stellar contract invoke --id $V2 --source budgetgate-verify --network testnet --build-only -- bump)
echo "[{\"name\":\"bump\",\"fixture_id\":\"counter_bump\",\"xdr\":\"$X2\"}]" > fixture_v2.json

cp contracts/counter/src/lib_v1.rs contracts/counter/src/lib.rs
```

Then rerun core as above, write the two snapshots back here with `--output-json`, and update the contract IDs and the result table. `target/` is ignored.
