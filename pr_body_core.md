# Execute Remediation Roadmap: Part A (Core)

This PR implements all fixes outlined in Part A of the `remediation-roadmap.md`, bringing `soroban-budget-core` out of a mocked state and into a fully functional, live-network integrated CLI tool.

## 🚀 Key Changes

### 1. Killed the Mock Path (A1)
- The `--fixture` branch no longer ignores the actual fixture file or returns a hardcoded dummy snapshot.
- The tool now iteratively parses a real JSON `fixture.json` file.
- It dynamically calls `SimulateTransaction` for each XDR entry provided in the fixture against the live Soroban RPC node.
- Real `Snapshot` structures are built from the responses, generating a legitimate `newSnapshot`.

### 2. Live Resource Extraction (A2)
- Extracted `ReadBytes` and `WriteBytes` footprints in addition to `CPUInstructions` directly from `transactionData`.
- Engineered a lightweight JS fallback `parse_xdr.js` parser to decode `SorobanTransactionData` base64 XDRs, fully side-stepping a massive `go-stellar-sdk` toolchain/proxy block that previously broke builds.

### 3. Threshold Calibration & Percentage Checks (A3)
- Formalized the maximum resource regression allowed via a new `--max-delta-pct` flag (defaulting to 10.0%).
- Reconciled mismatching documentation to reflect this configurability.

### 4. Network Absolute-Cap Warnings (A4)
- Implemented absolute protocol threshold caps (e.g. 100M CPU Instructions, 40MB Memory, etc.).
- Introduced a 80% Absolute-Cap safety warning `⚠️ CAP WARNING` alongside the regression checker `❌ REGRESSION` within the generated markdown table. Maintainers now get proactive alerts if a contract is getting dangerously close to protocol ceilings, even if the commit delta was small.

### 5. Snapshot Persistence (Action prerequisite)
- Added a new `--output-json` flag to export the freshly constructed `newSnapshot` back to disk, enabling `soroban-budget-action` to persist this as a baseline cache for subsequent pull requests.

## 🧪 Verification
- Evaluated end-to-end against a `balance_check` native asset invocation on `https://soroban-testnet.stellar.org`.
- Successfully validated that regressions and absolute limits correctly trigger non-zero exit codes.
