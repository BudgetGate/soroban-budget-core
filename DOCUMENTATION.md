# Soroban Budget Core - Documentation

`soroban-budget-core` is a high-performance Go CLI tool designed to execute Soroban smart contracts, monitor their resource consumption (CPU instructions and Memory bytes), and detect regressions against a baseline.

## Architecture & Features
*   **Dynamic Simulation**: Leverages Soroban RPC endpoints to dynamically trace transaction execution footprints.
*   **Regression Engine**: Compares newly generated snapshots against predefined baseline JSON files.
*   **Threshold Alerts**: Automatically detects and highlights resource spikes in `CPUInstructions`, `MemoryBytes`, `ReadBytes`, and `WriteBytes`.

## Deep Technical Details

### XDR Parsing
The CLI decodes the base64 `result.transactionData` returned by `simulateTransaction` into a `SorobanTransactionData` struct using the `xdr` package of the Stellar Go SDK (`github.com/stellar/go-stellar-sdk`). CPU instructions, disk read bytes and write bytes come from its `resources` field.

The instruction count the RPC declares is not raw usage: it is actual usage plus `max(instructionLeeway, minimum pad)`, where the minimum pad is set by the RPC provider (50,000 on soroban-testnet). To report actual usage, the CLI simulates each fixture twice, with `resourceConfig.instructionLeeway` set to 1,000,000 and 2,000,000. If the two declared values differ by exactly 1,000,000, the leeway was applied as-is and actual usage is the first value minus 1,000,000. If they don't, the provider pads differently and the CLI exits non-zero rather than report a wrong figure. Each fixture therefore costs two RPC calls.

Memory is not part of `SorobanTransactionData`. It was only exposed through the legacy `result.cost.memBytes` field, which current RPC versions omit. When it is absent, `memory_bytes` is recorded as `null` and is never treated as a regression.

A simulation that fails (the RPC returns `result.error`) or returns no `transactionData` makes the CLI exit non-zero instead of producing zero values.

### Threshold Calculations
Threshold calculations form the core of the Regression Engine. The engine takes two maps: `baseline` and `current`.
For each resource (e.g., `CPUInstructions`), the algorithm calculates the delta: `Delta = Current - Baseline`.
If `Delta > 0`, it computes the percentage increase: `Increase(%) = (Delta / Baseline) * 100`.
A regression is flagged if the `Increase(%)` exceeds a user-defined threshold limit (default is 5%). This provides a safeguard against unnoticed efficiency degradations in smart contract code before deployment.

### Markdown Diff Formatting
When the tool detects a regression or a successful check, it auto-generates a Markdown formatted table report. 
It uses a built-in text templating engine to format rows aligned with standard GitHub Flavored Markdown (GFM) tables.
Green checkmarks (`✅`) and Red crosses (`❌`) are dynamically inserted based on threshold validations. This makes the output instantly readable when piped to PR comments via GitHub Actions or GitLab CI.

## CLI Usage

The tool operates by parsing JSON configurations and simulating smart contract executions on the testnet.

### Flags
*   `--baseline` *(required)*: Path to the baseline JSON snapshot file containing the historically acceptable resource costs.
*   `--fixture` *(optional)*: Path to the JSON fixture file defining the contract execution parameters. If provided, the tool will dynamically run a simulation.
*   `--new` *(optional)*: Path to a static JSON snapshot file to compare against the baseline. (Mutually exclusive with `--fixture`).
*   `--rpc-url` *(optional)*: The Soroban RPC endpoint to use during dynamic simulations. Defaults to `https://soroban-testnet.stellar.org`.

### Examples

**Static File Comparison**
```bash
./budget-core --baseline ./history/base.json --new ./history/latest.json
```

**Dynamic Fixture Execution**
```bash
./budget-core --baseline ./history/base.json --fixture ./fixtures/swap_test.json --rpc-url https://soroban-testnet.stellar.org
```

## Output
The CLI automatically prints a markdown-formatted table to `stdout` detailing the differences. It exits with code `1` if a regression is detected, making it perfect for CI/CD environments.
