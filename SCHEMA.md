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
