# Baseline Snapshot Schema (v2.0)

This schema represents the JSON contract output by `soroban-budget-core`. It is consumed by both the GitHub Action and the Dashboard.

```json
{
  "schema_version": "2.0",
  "network": "testnet",
  "generated_at": "2026-08-06T12:00:00Z",
  "commit_sha": "abc1234",
  "functions": [
    {
      "name": "mint",
      "fixture_id": "test1",
      "cpu_instructions": 1000,
      "memory_bytes": null,
      "read_bytes": 200,
      "write_bytes": 100,
      "read_entries": 3,
      "write_entries": 2,
      "pct_of_network_cap": 0.05
    }
  ]
}
```

### Fields
- `schema_version`: Required. The version of this schema the snapshot follows (see [Versions](#versions)).
- `network`: The Soroban network this was tested against (e.g., `testnet`, `mainnet`).
- `generated_at`: ISO8601 timestamp.
- `commit_sha`: The commit hash of the contract being profiled.
- `functions`: An array of profiling results for specific invoked functions.
- `functions[].memory_bytes`: May be `null` when the RPC did not report memory usage (current `simulateTransaction` responses do not). Consumers must treat `null` as "unknown", not as `0`. `soroban-budget-core` does not currently compare or report this field.
- `functions[].read_bytes`: The RPC's `diskReadBytes`. Since protocol 23 this covers only classic and archived entries; reads of live contract storage are **not** included.
- `functions[].read_entries` / `functions[].write_entries`: Number of ledger entries in the transaction footprint (all entries / read-write entries). These are the signal for added contract-storage access. They are `null` (or absent) in snapshots written before they were introduced, and comparisons against such a baseline show them as not reported.

## Versions

| Version | Changes |
|---|---|
| `2.0` | `memory_bytes` may be `null`. Added `read_entries` and `write_entries`. `cpu_instructions` is actual usage, not the RPC's padded estimate. |
| `1.0` | First version. `memory_bytes` was always a number; no entry counts. |

Core writes `2.0`. Snapshots written by core before this version was introduced say `1.0` even when they already contain `2.0` fields.

**Reading.** `soroban-budget-core` reads `1.0` and `2.0` snapshots, as a baseline or with `--new`. A `1.0` snapshot compares cleanly: memory is not compared, and missing entry counts are reported as unavailable. A snapshot with a missing or any other `schema_version` is rejected with an error on stderr and exit code `2`.

**Consumers** (the Action, the dashboard, other tools) must check `schema_version` before using a snapshot and fail with a clear message on a version they do not support. Do not guess at an unknown version's fields.

**Changing the schema.** Any change to a field's name, type or meaning bumps `schema_version`, updates this file, and states the impact on the Action and the dashboard in the PR description.

## Fixture Schema (Input)

The fixture JSON file contains an array of inputs for simulation:

```json
[
  {
    "name": "mint",
    "fixture_id": "test1",
    "xdr": "AAAAAgAAAABdummyXDR..."
  }
]
```

### Fields
- `name`: The logical name of the function or test.
- `fixture_id`: A unique identifier for this fixture item.
- `xdr`: The base64-encoded XDR of the transaction envelope.
