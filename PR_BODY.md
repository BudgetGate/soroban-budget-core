## 🚀 Background
The original `SPEC.md` for the BudgetGate project required the `soroban-budget-core` engine to dynamically execute Soroban smart contracts using a fixture definition and pinging the Soroban RPC directly, rather than just comparing two pre-generated JSON files.

## 🛠️ Problem
In the MVP phase, `main.go` was simplified to only accept `--baseline` and `--new` JSON files. While `rpc.go` contained the `SimulateTransaction` logic, it was completely unutilized by the CLI, meaning the core engine lacked the ability to dynamically simulate and construct the `new.json` state.

## ✨ Solution & Implementation
This PR completes the missing specification by fully integrating the RPC simulation logic into the CLI execution path.
- **New Flags:** Introduced `--fixture` (to specify a path to the JSON fixture defining the contract calls) and `--rpc-url` (to specify the Soroban RPC endpoint).
- **Dynamic Simulation:** When `--fixture` is passed instead of `--new`, the engine will now invoke `SimulateTransaction` to ping the Soroban network.
- **State Construction:** The response from the RPC node is dynamically parsed to construct the new `Snapshot` struct in-memory before passing it to the diffing engine.

## 📋 Testing
- [x] Verified CLI argument parsing (ensures mutual exclusivity of `--new` and `--fixture`).
- [x] Tested the mock execution path with dummy XDR to ensure it generates the snapshot properly.
