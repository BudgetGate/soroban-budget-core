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

Testnet is reset periodically, which removes the contracts and the fixtures' source account; simulation then fails. The weekly `Counter check` workflow (`.github/workflows/counter-check.yml`) measures both fixtures and compares them with the committed snapshots, so a reset shows up as a failed run. To recover, from the repository root:

```bash
testdata/counter/redeploy.sh                 # replaces the fixtures and snapshots here
testdata/counter/redeploy.sh /tmp/counter    # or write them elsewhere to look first
```

It funds (or creates) the `budgetgate-verify` key, builds and deploys v1 and v2, writes both fixtures with `stellar contract invoke --build-only`, regenerates both snapshots with core, and checks that core exits 1 on v2. Each testnet step is tried up to three times, since the RPC sometimes times out. Afterwards, update the contract IDs above and the result table if the numbers moved, and copy `fixture_v1.json` to `budget/fixture.json` in `BudgetGate/budgetgate-action-test`. `target/` is ignored.

Checked on 2026-10-29: the script deployed new contracts (v1 `CCF3UEF6LIHRLGRV6XZKHGBG5JN7MV2Z253CHYPCCGT7CM45Z5TODNMW`, v2 `CAM6NSDDIAWKIDDBRI7YNE2QAOEYMCDCIL4BQY7ZICRLWL5PIUBXVEM3`) into a scratch directory, and their snapshots matched the committed ones exactly.
