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
- `network`: The network the snapshot was measured on, from the RPC's `getNetwork` passphrase: `testnet`, `mainnet` or `futurenet`, or the passphrase itself for any other network.
- `generated_at`: When the snapshot was measured, RFC 3339 in UTC (e.g. `2026-10-26T23:21:33Z`).
- `commit_sha`: The commit of the contract code that was measured, as passed with `--commit-sha`. Empty when not given.

Snapshots written before core filled these fields have `network` `"testnet"` (hardcoded), `generated_at` `""` and `commit_sha` `"dynamic"`.
- `functions`: An array of profiling results for specific invoked functions.
- `functions[].name`: The name a function is compared by: a function in the new snapshot is compared with the baseline function of the same name. Unique within a snapshot (see [Reading](#versions)).
- `functions[].cpu_instructions`: The instructions the RPC's simulation used, with its declaration pad removed. Executing the same call uses slightly fewer: on testnet (protocol 29, 2026-10-29) three calls simulated at 291,573, 411,511 and 559,224 used 288,297, 409,658 and 555,007 when submitted (0.45%–1.14% less, from the executed transactions' `core_metrics` `cpu_insn`). Both sides of a comparison are simulated, so comparisons are unaffected; the value is not what a transaction declares or pays for.
- `functions[].memory_bytes`: May be `null` when the RPC did not report memory usage (current `simulateTransaction` responses do not). Consumers must treat `null` as "unknown", not as `0`. `soroban-budget-core` does not currently compare or report this field.
- `functions[].read_bytes`: The RPC's `diskReadBytes`. Since protocol 23 this covers only classic and archived entries; reads of live contract storage are **not** included.
- `functions[].read_entries` / `functions[].write_entries`: Number of ledger entries in the transaction footprint (all entries / read-write entries). These are the signal for added contract-storage access. They are `null` (or absent) in snapshots written before they were introduced, and comparisons against such a baseline show them as not reported.
- `functions[].pct_of_network_cap`: The highest percentage of any metric's per-transaction network limit that the function uses (CPU of `txMaxInstructions`, read bytes of `txMaxDiskReadBytes`, write bytes of `txMaxWriteBytes`, read entries of `txMaxFootprintEntries`, write entries of `txMaxWriteLedgerEntries`), with the limits read from the network when the snapshot was measured. `0` in snapshots written before core read network caps, and in snapshots not built from a fixture. Informational: comparisons use the live caps, not this field.

## Versions

| Version | Changes |
|---|---|
| `2.0` | `memory_bytes` may be `null`. Added `read_entries` and `write_entries`. `cpu_instructions` is what the simulation used, not the RPC's padded estimate (an executed call uses slightly less; see `functions[].cpu_instructions`). |
| `1.0` | First version. `memory_bytes` was always a number; no entry counts. |

Core writes `2.0`. Snapshots written by core before this version was introduced say `1.0` even when they already contain `2.0` fields.

**Reading.** `soroban-budget-core` reads `1.0` and `2.0` snapshots, as a baseline or with `--new`. A `1.0` snapshot compares cleanly: memory is not compared, and missing entry counts are reported as unavailable. A snapshot with a missing or any other `schema_version` is rejected with an error on stderr and exit code `2`. A new snapshot (`--new`) that has two functions with the same `name` is rejected the same way. A baseline that has one (core did not check names before; a fixture that called one function twice under its name wrote such a baseline) is read with a warning on stderr, and the functions with that name are compared as new, since either entry would be a guess; the next snapshot saved as the baseline no longer has the duplicate.

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
- `name`: The logical name of the function or test, written to the snapshot as `functions[].name`. Must be unique within the fixture, since the baseline is matched by it; a fixture with a repeated `name` is rejected with exit code `2` before anything is simulated. To measure one function with different arguments, give each call its own name.
- `fixture_id`: A unique identifier for this fixture item.
- `xdr`: The base64-encoded XDR of the transaction envelope.
