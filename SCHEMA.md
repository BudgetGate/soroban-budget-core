# Baseline Snapshot Schema (v1.0)

This schema represents the JSON contract output by `soroban-budget-core`. It is consumed by both the GitHub Action and the Dashboard.

```json
{
  "schema_version": "1.0",
  "network": "testnet",
  "generated_at": "2026-08-06T12:00:00Z",
  "commit_sha": "abc1234",
  "functions": [
    {
      "name": "mint",
      "fixture_id": "test1",
      "cpu_instructions": 1000,
      "memory_bytes": 500,
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
- `schema_version`: Used for backwards compatibility.
- `network`: The Soroban network this was tested against (e.g., `testnet`, `mainnet`).
- `generated_at`: ISO8601 timestamp.
- `commit_sha`: The commit hash of the contract being profiled.
- `functions`: An array of profiling results for specific invoked functions.
- `functions[].memory_bytes`: May be `null` when the RPC did not report memory usage (current `simulateTransaction` responses do not). Consumers must treat `null` as "unknown", not as `0`. `soroban-budget-core` does not currently compare or report this field.
- `functions[].read_bytes`: The RPC's `diskReadBytes`. Since protocol 23 this covers only classic and archived entries; reads of live contract storage are **not** included.
- `functions[].read_entries` / `functions[].write_entries`: Number of ledger entries in the transaction footprint (all entries / read-write entries). These are the signal for added contract-storage access. They are `null` (or absent) in snapshots written before they were introduced, and comparisons against such a baseline show them as not reported.

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
