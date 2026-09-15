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

The CLI takes two primary arguments: a path to a baseline JSON file and a path to the new JSON file you want to evaluate.

```bash
./budget-core --baseline path/to/baseline.json --new path/to/new.json
```

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