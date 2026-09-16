# Soroban Budget Guard - Core Engine

`soroban-budget-core` is the foundational Go-based CLI tool for the **BudgetGate** suite. It parses snapshot data from Soroban smart contract simulations, compares current resource usage against a baseline, detects regressions, and outputs a formatted Markdown report.

## 🚀 Features

- **Resource Tracking:** Compares CPU instructions, memory bytes, and read/write bytes.
- **Regression Detection:** Automatically identifies resource regressions when a new commit exceeds baseline costs by more than 10%.
- **CI/CD Ready:** Exits with `0` on success and `1` (or another non-zero code) on regressions, making it perfect for automated workflows.
- **Markdown Reporting:** Outputs a clean, GitHub-flavored Markdown table summarizing the differences.

## 📦 Installation

Ensure you have [Go](https://golang.org/dl/) 1.20 or later installed.

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

Example:
```bash
./budget-core --baseline baseline.json --fixture fixture.json --rpc-url https://soroban-testnet.stellar.org --max-delta-pct 10.0
```

The CLI will produce a Markdown formatted table comparing the costs and surfacing regressions if they exceed the specified `--max-delta-pct` or reach a hardcoded network absolute cap (e.g. 100M CPU, 40MB Memory).

### Example Output

```markdown
| Function | Metric | Baseline | New | Delta | % Change | Status |
|----------|--------|----------|-----|-------|----------|--------|
| mint | CPUInstructions | 1000 | 1100 | 100 | 10.00% | ❌ REGRESSION |
| mint | MemoryBytes | 500 | 500 | 0 | 0.00% | ✅ OK |
| mint | ReadBytes | 200 | 210 | 10 | 5.00% | ✅ OK |
| mint | WriteBytes | 100 | 100 | 0 | 0.00% | ✅ OK |
```

## 📄 JSON Schema

The tool expects snapshots to follow a specific JSON schema. See the [`SCHEMA.md`](./SCHEMA.md) file for the exact data structure required.

## 🧪 Development & Testing

To run the internal test suite:

```bash
go test ./... -v
```

## 🤝 Integration

This core binary is designed to be used independently or wrapped by our GitHub Action (`soroban-budget-action`) for seamless CI/CD integration.