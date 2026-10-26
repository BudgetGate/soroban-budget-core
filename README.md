# Soroban Budget Guard - Core Engine

`soroban-budget-core` is the foundational Go-based CLI tool for the **BudgetGate** suite. It parses snapshot data from Soroban smart contract simulations, compares current resource usage against a baseline, detects regressions, and outputs a formatted Markdown report.

## 🚀 Features

- **Resource Tracking:** Compares CPU instructions, read/write bytes, and the number of ledger entries read and written. Memory is not compared: current Soroban RPC versions no longer report it from `simulateTransaction`.
- **Regression Detection:** Automatically identifies resource regressions when a new commit exceeds baseline costs by more than 10%.
- **CI/CD Ready:** Distinct exit codes, so a regression is never confused with a run that failed (see [Exit codes](#exit-codes)).
- **Markdown Reporting:** Outputs a clean, GitHub-flavored Markdown table summarizing the differences.

## 📦 Installation

Ensure you have [Go](https://golang.org/dl/) 1.25 or later installed. No other runtime is needed; XDR is decoded natively with the Stellar Go SDK.

```bash
# Clone the repository
cd soroban-budget-core

# Build the binary
go build -o budget-core
```

## 🛠️ Usage

1. Ensure you have a baseline generated.
2. Provide an XDR payload or fixture to test against via `--fixture` or an existing snapshot via `--new`.
3. Provide the testnet or mainnet RPC URL via `--rpc-url`.
4. (Optional) Tune the thresholds. Each one is off when set to `0`; negative values are rejected.

| Flag | Default | Effect |
|------|---------|--------|
| `--max-delta-pct` | `10` | Regression when a metric rises by more than this percentage. |
| `--max-cpu-increase` | `0` | Regression when CPU instructions rise by more than this many. |
| `--max-read-bytes-increase` | `0` | Same for read bytes. |
| `--max-write-bytes-increase` | `0` | Same for write bytes. |
| `--max-read-entries-increase` | `0` | Same for read entries. |
| `--max-write-entries-increase` | `0` | Same for write entries. |
| `--cap-warning-pct` | `80` | `⚠️ CAP WARNING` when a metric is above this percentage of the network's per-transaction cap. |
| `--fail-on-cap-warning` | off | Exit `1` when any metric has a cap warning. Without it, a warning never fails the run. Needs `--cap-warning-pct` above `0`. |

A metric regresses if it breaks either its percentage or its absolute limit.

### Network caps

The caps are read from the network on every run, from `--rpc-url`, so they match that network and its current protocol version. Nothing is hardcoded. The report ends with the protocol version and ledger they came from.

| Metric | Network limit |
|--------|---------------|
| CPUInstructions | `txMaxInstructions` |
| ReadBytes | `txMaxDiskReadBytes` |
| WriteBytes | `txMaxWriteBytes` |
| ReadEntries | `txMaxFootprintEntries` (all footprint entries, matching how `read_entries` is counted) |
| WriteEntries | `txMaxWriteLedgerEntries` |

If the caps can't be read, the run exits `2`. A `--new` run with `--cap-warning-pct 0` skips the lookup and works offline; `% of Cap` then shows `n/a`.

`testdata/fixture.json` targets soroban-testnet, and `testdata/snapshot_testnet.json` is a snapshot core measured from it. After a testnet reset, regenerate the fixture with `go run ./cmd/genfixture > testdata/fixture.json` (see [`testdata/README.md`](./testdata/README.md)).

Example:
```bash
./budget-core --baseline testdata/snapshot_testnet.json --fixture testdata/fixture.json --rpc-url https://soroban-testnet.stellar.org --max-delta-pct 10.0
```

The CLI produces a Markdown table comparing the costs, flagging regressions and metrics close to the network's caps.

### Example Output

Output of the example above on 2026-10-27 (exit code 0; the costs had not changed since the snapshot was taken):

```markdown
| Function | Metric | Baseline | New | Delta | % Change | % of Cap | Status |
|----------|--------|----------|-----|-------|----------|----------|--------|
| balance_check | CPUInstructions | 91406 | 91406 | 0 | 0.00% | 0.02% | ✅ OK |
| balance_check | ReadBytes | 144 | 144 | 0 | 0.00% | 0.07% | ✅ OK |
| balance_check | WriteBytes | 0 | 0 | 0 | 0.00% | 0.00% | ✅ OK |
| balance_check | ReadEntries | 2 | 2 | 0 | 0.00% | 0.50% | ✅ OK |
| balance_check | WriteEntries | 0 | 0 | 0 | 0.00% | 0.00% | ✅ OK |
| transfer_to_contract | CPUInstructions | 218663 | 218663 | 0 | 0.00% | 0.05% | ✅ OK |
| transfer_to_contract | ReadBytes | 144 | 144 | 0 | 0.00% | 0.07% | ✅ OK |
| transfer_to_contract | WriteBytes | 368 | 368 | 0 | 0.00% | 0.28% | ✅ OK |
| transfer_to_contract | ReadEntries | 3 | 3 | 0 | 0.00% | 0.75% | ✅ OK |
| transfer_to_contract | WriteEntries | 2 | 2 | 0 | 0.00% | 1.00% | ✅ OK |

% of Cap uses the network's per-transaction limits at protocol 29, ledger 5406551.
```

A row turns `❌ REGRESSION` when a metric rises more than `--max-delta-pct`, and the CLI then exits `1`.

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | No regression. Also `-h`. |
| `1` | At least one metric regressed, or a cap warning with `--fail-on-cap-warning`. |
| `2` | The run could not compare: bad or missing flags, unreadable or invalid files, an unsupported `schema_version`, an RPC failure, or a `--output-json` file that could not be written. |

stdout carries only the Markdown report, so it can be redirected or posted as-is. Usage, progress and error messages go to stderr; on an error, stdout is empty, except when only writing `--output-json` failed: the report is printed first, then the run exits `2`.

## 📄 JSON Schema

The tool expects snapshots to follow a specific JSON schema. See the [`SCHEMA.md`](./SCHEMA.md) file for the exact data structure required.

## 🧪 Development & Testing

To run the internal test suite:

```bash
go test ./... -v
```

## 🤝 Integration

This core binary is designed to be used independently or wrapped by our GitHub Action (`soroban-budget-action`) for seamless CI/CD integration.