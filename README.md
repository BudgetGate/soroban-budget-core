# Soroban Budget Guard - Core Engine

`soroban-budget-core` is the foundational Go-based CLI tool for the **BudgetGate** suite. It parses snapshot data from Soroban smart contract simulations, compares current resource usage against a baseline, detects regressions, and outputs a formatted Markdown report.

## 🚀 Features

- **Resource Tracking:** Compares CPU instructions, read/write bytes, and the number of ledger entries read and written. Memory is not compared: current Soroban RPC versions no longer report it from `simulateTransaction`.
- **Regression Detection:** Automatically identifies resource regressions when a new commit exceeds baseline costs by more than 10%.
- **CI/CD Ready:** Exits with `0` on success and `1` (or another non-zero code) on regressions, making it perfect for automated workflows.
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
4. (Optional) Provide `--max-delta-pct` (default is 10.0) to configure the regression threshold.

`testdata/fixture.json` targets soroban-testnet, and `testdata/snapshot_testnet.json` is a snapshot core measured from it. After a testnet reset, regenerate the fixture with `go run ./cmd/genfixture > testdata/fixture.json` (see [`testdata/README.md`](./testdata/README.md)).

Example:
```bash
./budget-core --baseline testdata/snapshot_testnet.json --fixture testdata/fixture.json --rpc-url https://soroban-testnet.stellar.org --max-delta-pct 10.0
```

The CLI will produce a Markdown formatted table comparing the costs and surfacing regressions if they exceed the specified `--max-delta-pct` or reach a hardcoded network absolute cap (e.g. 100M CPU instructions).

### Example Output

Output of the example above on 2026-10-26 (exit code 0; the costs had not changed since the snapshot was taken):

```markdown
| Function | Metric | Baseline | New | Delta | % Change | % of Cap | Status |
|----------|--------|----------|-----|-------|----------|----------|--------|
| balance_check | CPUInstructions | 91406 | 91406 | 0 | 0.00% | 0.09% | ✅ OK |
| balance_check | ReadBytes | 144 | 144 | 0 | 0.00% | 0.00% | ✅ OK |
| balance_check | WriteBytes | 0 | 0 | 0 | 0.00% | 0.00% | ✅ OK |
| balance_check | ReadEntries | 2 | 2 | 0 | 0.00% | 0.00% | ✅ OK |
| balance_check | WriteEntries | 0 | 0 | 0 | 0.00% | 0.00% | ✅ OK |
| transfer_to_contract | CPUInstructions | 218663 | 218663 | 0 | 0.00% | 0.22% | ✅ OK |
| transfer_to_contract | ReadBytes | 144 | 144 | 0 | 0.00% | 0.00% | ✅ OK |
| transfer_to_contract | WriteBytes | 368 | 368 | 0 | 0.00% | 0.04% | ✅ OK |
| transfer_to_contract | ReadEntries | 3 | 3 | 0 | 0.00% | 0.00% | ✅ OK |
| transfer_to_contract | WriteEntries | 2 | 2 | 0 | 0.00% | 0.00% | ✅ OK |
```

A row turns `❌ REGRESSION` when a metric rises more than `--max-delta-pct`, and the CLI then exits `1`.

## 📄 JSON Schema

The tool expects snapshots to follow a specific JSON schema. See the [`SCHEMA.md`](./SCHEMA.md) file for the exact data structure required.

## 🧪 Development & Testing

To run the internal test suite:

```bash
go test ./... -v
```

## 🤝 Integration

This core binary is designed to be used independently or wrapped by our GitHub Action (`soroban-budget-action`) for seamless CI/CD integration.