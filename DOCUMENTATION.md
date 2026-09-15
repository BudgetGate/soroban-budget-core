# Soroban Budget Core - Documentation

`soroban-budget-core` is a high-performance Go CLI tool designed to execute Soroban smart contracts, monitor their resource consumption (CPU instructions and Memory bytes), and detect regressions against a baseline.

## Architecture & Features
*   **Dynamic Simulation**: Leverages Soroban RPC endpoints to dynamically trace transaction execution footprints.
*   **Regression Engine**: Compares newly generated snapshots against predefined baseline JSON files.
*   **Threshold Alerts**: Automatically detects and highlights resource spikes in `CPUInstructions`, `MemoryBytes`, `ReadBytes`, and `WriteBytes`.

## Deep Technical Details

### XDR Parsing
The Go CLI interacts with the Soroban network by encoding and decoding Stellar XDR (External Data Representation). It utilizes the stellar/go SDK to strictly parse Base64 encoded transaction envelopes and simulation results.
When fetching simulation data via RPC, the `result.transactionData` and `result.events` are received as base64-encoded XDR strings. The parser converts these into native Go structs representing `TransactionMeta`, extracting precise metrics for CPU Instructions and Memory usage injected by the Soroban environment. 

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
