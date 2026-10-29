<p align="center"><img src=".github/avatar.png" width="120" alt="Budget Gate logo: a resource-cost gauge behind a gate bar"></p>

# Soroban Budget Guard - Core Engine

`soroban-budget-core` is the Go CLI at the heart of the **BudgetGate** suite. It simulates Soroban contract calls with the RPC method `simulateTransaction`, compares the measured resource cost against a baseline snapshot, and prints a Markdown report.

**Documentation:** [budgetgate.github.io/docs](https://budgetgate.github.io/docs/core/). This README covers building and running core; the docs site explains how the numbers are measured.

> **Status:** early-stage. Not audited and not production-ready. Latest release: [v0.2.3](https://github.com/BudgetGate/soroban-budget-core/releases/tag/v0.2.3) (Linux, macOS and Windows; x86-64 and arm64).

## 🚀 Features

- **Resource Tracking:** Compares CPU instructions, read/write bytes, and the number of ledger entries read and written. Memory is not compared: current Soroban RPC versions no longer report it from `simulateTransaction`.
- **Regression Detection:** Flags a metric that rises by more than a percentage (10% by default) or by more than an absolute amount you set. Functions added to or removed from the fixture are reported too.
- **Network Caps:** Shows how close each metric is to the network's per-transaction limit, read live from the RPC.
- **CI/CD Ready:** Distinct exit codes, so a regression is never confused with a run that failed (see [Exit codes](#exit-codes)).
- **Markdown Reporting:** Outputs a clean, GitHub-flavored Markdown table summarizing the differences.

## 📦 Installation

### Download a release

Each release has static binaries for Linux, macOS and Windows on x86-64 and arm64, plus one `SHA256SUMS` file covering them: `budget-core-linux-amd64`, `budget-core-linux-arm64`, `budget-core-darwin-amd64`, `budget-core-darwin-arm64`, `budget-core-windows-amd64.exe`, `budget-core-windows-arm64.exe`.

Linux and macOS (pick your `ASSET`):

```bash
V=v0.2.3; ASSET=budget-core-linux-amd64
curl -sSLO https://github.com/BudgetGate/soroban-budget-core/releases/download/$V/$ASSET
curl -sSLO https://github.com/BudgetGate/soroban-budget-core/releases/download/$V/SHA256SUMS
sha256sum -c --ignore-missing SHA256SUMS      # macOS: shasum -a 256 -c --ignore-missing SHA256SUMS
chmod +x $ASSET && mv $ASSET budget-core
```

Windows (PowerShell; the comparison must print `True`):

```powershell
$V = 'v0.2.3'; $A = 'budget-core-windows-amd64.exe'
$R = "https://github.com/BudgetGate/soroban-budget-core/releases/download/$V"
Invoke-WebRequest "$R/$A" -OutFile $A; Invoke-WebRequest "$R/SHA256SUMS" -OutFile SHA256SUMS
$expected = ((Select-String -Path SHA256SUMS -Pattern " $A$").Line -split '\s+')[0]
(Get-FileHash $A -Algorithm SHA256).Hash.ToLower() -eq $expected
Rename-Item $A budget-core.exe
```

`--ignore-missing` is needed because `SHA256SUMS` lists all six binaries. v0.2.1 and earlier only include `budget-core-linux-amd64`.

Releases after v0.2.3 also carry a signed build provenance attestation for each binary, which proves it was built by this repository's release workflow from the tagged commit. Check it with the GitHub CLI before running the binary (v0.2.3 and earlier have none, so this fails for them):

```bash
gh attestation verify budget-core -R BudgetGate/soroban-budget-core
```

### Build from source

Ensure you have [Go](https://golang.org/dl/) 1.26.10 or later installed (`go.mod` requires it; with an older Go, the `go` command downloads 1.26.10 itself unless `GOTOOLCHAIN=local`). No other runtime is needed; XDR is decoded natively with the Stellar Go SDK.

```bash
git clone https://github.com/BudgetGate/soroban-budget-core.git
cd soroban-budget-core
go build -o budget-core .
```

## 🛠️ Usage

1. Provide a baseline snapshot with `--baseline`. For the first run, start from an empty one and save the result as your baseline:
   ```bash
   echo '{"schema_version":"2.0","functions":[]}' > empty.json
   ./budget-core --baseline empty.json --fixture fixture.json --output-json baseline.json
   ```
2. Provide a [fixture](./SCHEMA.md#fixture-schema-input) of contract calls to simulate with `--fixture`, or an existing snapshot with `--new`.
3. Provide a Soroban RPC URL via `--rpc-url` (default `https://soroban-testnet.stellar.org`). Any Soroban RPC that serves `simulateTransaction`, `getLedgerEntries` and `getNetwork` works. Contract changes have been measured on testnet. On mainnet (2026-10-30, v0.2.3), a read-only call (the native XLM contract's `balance`) measured the same on three public RPC providers (`https://mainnet.sorobanrpc.com`, `https://soroban-rpc.mainnet.stellar.gateway.fm`, `https://rpc.ankr.com/stellar_soroban`): 91,406 CPU instructions, network caps read at protocol 29, `network` recorded as `mainnet`. Simulating is free and changes nothing, but the source account in each fixture entry must exist on that network.
4. (Optional) With `--output-json`, pass `--commit-sha` to record which commit of your contract was measured. The snapshot also records the network (read from the RPC) and when it was generated.
5. (Optional) Tune the thresholds. Each one is off when set to `0`; negative values are rejected.

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

A row turns `❌ REGRESSION` when a metric breaks its percentage or absolute limit, and the CLI then exits `1`. All statuses, including `🆕 NEW` and `🗑️ REMOVED` functions, are listed in the [report reference](https://budgetgate.github.io/docs/core/api-reference#report).

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | No regression. Also `-h`. |
| `1` | At least one metric regressed, or a cap warning with `--fail-on-cap-warning`. |
| `2` | The run could not compare: bad or missing flags, unreadable or invalid files, an unsupported `schema_version`, a fixture or `--new` snapshot that repeats a function name, an RPC failure, or a `--output-json` file that could not be written. |

stdout carries only the Markdown report, so it can be redirected or posted as-is. Usage, progress and error messages go to stderr; on an error, stdout is empty, except when only writing `--output-json` failed: the report is printed first, then the run exits `2`.

## 📄 JSON Schema

The tool expects snapshots to follow a specific JSON schema. See the [`SCHEMA.md`](./SCHEMA.md) file for the exact data structure required.

## 🧪 Development & Testing

```bash
gofmt -l .        # must print nothing
go vet ./...
go test ./...     # no network access needed
```

See [`CONTRIBUTING.md`](./CONTRIBUTING.md) before opening a pull request.

## 🤝 Integration

This core binary is designed to be used independently or wrapped by our GitHub Action (`soroban-budget-action`) for seamless CI/CD integration.
## 🔒 Security

Report vulnerabilities privately; see [`SECURITY.md`](./SECURITY.md).
