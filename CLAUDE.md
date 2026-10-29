# CLAUDE.md — soroban-budget-core

Scope: this repo only. If the workspace-level `CLAUDE.md` exists in the parent folder, read it as well. Where the two differ, this file wins for this repo. The defect table below was checked against `origin/main` (`6538c03`) on 2026-10-29. Re-verify an item before relying on it.

## 1. What this repo is

The engine of Budget Gate. A Go CLI that takes a baseline snapshot of Soroban contract resource costs and a new snapshot, compares them, and prints a Markdown report. Its second job is to produce the new snapshot itself by calling the Soroban RPC method `simulateTransaction` (`--fixture`). That path makes real RPC calls and was verified against a deployed contract (section 3).

User docs live in the separate `BudgetGate/docs` site (budgetgate.github.io/docs/), which describes this repo as of `6538c03` (each user-facing PR since #12 shipped with a matching docs PR; the Go version and v0.2.1 in docs #37 and #38). A PR here that changes flags, exit codes, output or the schema needs a matching docs PR, linked from both descriptions.

This repo owns the snapshot schema. The Action and the dashboard both depend on it. It must work standalone, outside GitHub.

Boundaries:
- No GitHub API code, no reference to `soroban-budget-action` or `soroban-budget-dashboard`.
- The only network calls allowed are to a Soroban RPC endpoint.
- Not a static analyzer, not a security scanner. Scout-Soroban already covers that.

## 2. Known layout

Layout last checked in full against `main` `c060e90` (after #4–#23) on 2026-10-27; later PRs (through #35, `6538c03`, 2026-10-29) updated the rows they changed. Module `github.com/budgetgate/soroban-budget-core`, `go 1.26.10` (C19; was `go 1.25`). Direct dependencies: `github.com/stellar/go-stellar-sdk v0.8.0` (XDR decoding in `rpc.go`, `txnbuild` in `cmd/genfixture`), `github.com/stretchr/testify v1.12.1` (both bumped by Dependabot in #44).

| File | Role (read in full unless stated) |
|------|------|
| `main.go` | Flags `--baseline` (required), `--new`, `--fixture` (wins over `--new`), `--rpc-url` (default `https://soroban-testnet.stellar.org`), `--output-json`, `--commit-sha` (recorded in fixture-built snapshots), `--max-delta-pct` (default 10), `--cap-warning-pct` (default 80), `--fail-on-cap-warning` (exit 1 on any cap warning; needs `--cap-warning-pct` > 0), and one absolute-increase flag per metric (`--max-cpu-increase`, `--max-read-bytes-increase`, `--max-write-bytes-increase`, `--max-read-entries-increase`, `--max-write-entries-increase`; default 0 = off). Negative thresholds exit 2. `Thresholds.MaxIncrease` is keyed by metric name. Reads network caps with `FetchNetworkCaps` before comparing (skipped for `--new` with `--cap-warning-pct 0`); a failed lookup exits 2. Function names must be unique (C22): a fixture or `--new` snapshot that repeats one exits 2 (the fixture before any simulation); a baseline that repeats one gets a stderr warning and those functions are dropped from it, so they show as `🆕 NEW`. With `--fixture` it calls `MeasureResources` per entry, sets `pct_of_network_cap`, and builds the snapshot; with `--new` it reads one. After the table, stdout names the protocol version and ledger the caps came from. Exit codes 0 pass, 1 regression, 2 usage or runtime error (`exitOK`, `exitRegression`, `exitError`). Logic lives in `run(args, stdout, stderr) int`, so tests call it directly; `main` only calls `os.Exit`. stdout carries only the Markdown report; usage, progress and errors go to stderr. |
| `rpc.go` | `MeasureResources(ctx, rpcURL, txXDR)`: simulates twice with `instructionLeeway` 1,000,000 and 2,000,000, checks the declared values differ by exactly 1,000,000, and reports actual CPU = first − 1,000,000. All RPC calls go through `postJSONRPC`: an `http.Client` with a 30 s timeout (`rpcTimeout`), `NewRequestWithContext`, and a failure on any non-200 status with `rpc returned HTTP <code>: <body snippet>`. JSON-RPC errors are `*rpcError`. `ParseTransactionData` decodes `transactionData` into `Resources` (instructions, disk read bytes, write bytes, `ReadEntries`, `WriteEntries`). Fails on `result.error` or missing `transactionData`. `SimulateTransaction` is kept for other callers. |
| `caps.go` | `FetchNetworkCaps(ctx, rpcURL)`: one `getLedgerEntries` call for the `CONTRACT_COMPUTE_V0`, `CONTRACT_LEDGER_COST_V0` and `CONTRACT_LEDGER_COST_EXT_V0` config settings, one `getNetwork` call for the protocol version. Maps CPU → `TxMaxInstructions`, read bytes → `TxMaxDiskReadBytes`, write bytes → `TxMaxWriteBytes`, read entries → `TxMaxFootprintEntries` (all footprint entries, as `read_entries` counts them), write entries → `TxMaxWriteLedgerEntries`. Errors if any limit is missing. |
| `diff.go` | `Thresholds` (including `NetworkCaps`), `DiffResult` (`HasCap`; `% of Cap` shows `n/a` without a cap), `CompareSnapshots` (CPU, read bytes, write bytes, read entries, write entries; memory not compared; added and removed functions), `capUsage`, `pctOfNetworkCap`, `FormatDiffs`. |
| `snapshot.go` | `FixtureEntry`, `Snapshot`, `FunctionCost` (`MemoryBytes`, `ReadEntries`, `WriteEntries` are `*int64`), `ParseSnapshot` (rejects a missing or unknown `schema_version` with `ErrUnsupportedSchema`), `CurrentSchemaVersion` (`"2.0"`), and the C22 helpers `duplicateNames` (generic; each repeated name once), `functionName`, `withoutNames`, `quotedList`. |
| `cmd/genfixture/main.go` | Regenerates `testdata/fixture.json` against testnet: funds an account via friendbot, builds signed `balance` and `transfer` calls on the native-asset contract. `go run ./cmd/genfixture > testdata/fixture.json`. |
| `*_test.go` | testify, table-driven: 23 test functions (145 passing tests and subtests in `go test -v`, 2026-10-29, after C22), no network access. Parser tests read responses captured from testnet (`testdata/simulate_*.json`, `getledgerentries_config.json`, `getnetwork.json`). `paddingRPC` fakes the RPC (two-leeway simulation plus the captured cap responses); CLI tests point `--rpc-url` at it unless a case sets its own. `TestTestdataSamples` keeps the sample files readable. `TestCounterContractSnapshots` compares the counter contract's real v1 and v2 snapshots offline (exit 1, every row). |
| `SCHEMA.md` | Snapshot format (v2.0, with a version history and the reading rules) and fixture format (`[{name, fixture_id, xdr}]`). Authoritative. |
| `testdata/` | `fixture.json` (`balance_check` read-only, `transfer_to_contract` writes storage; testnet native-asset contract), `simulate_*.json` (captured responses), `getledgerentries_config.json` and `getnetwork.json` (captured cap and protocol responses, protocol 29), `snapshot_testnet.json` (real snapshot, schema 2.0, with `pct_of_network_cap`), `counter/` (the counter contract v1/v2 used for rule 3, its testnet fixtures and the snapshots core measured from them; rerun and redeploy steps in `counter/README.md`; `counter/target/` is ignored). Provenance and regeneration in `testdata/README.md`. |
| `.github/workflows/ci.yml` | `Core CI`, job `build-and-test`: `go build`, `go test`. Go version from `go.mod`. `actions/checkout@v7`, `actions/setup-go@v7` (both `node24`). Runs on push/PR to `main` only. |
| `.github/workflows/counter-check.yml` | `Counter check` (C21). Weekly (Monday 06:17 UTC), on demand, and on a PR that changes it: builds core, measures `testdata/counter/fixture_v1.json` (expects exit 0) and `fixture_v2.json` against it (expects exit 1) on testnet, and diffs both snapshots' `functions` with the committed ones. A simulation failure points to `testdata/counter/redeploy.sh` (testnet reset); a diff means the measured cost or the network limits changed. GitHub disables scheduled workflows in a public repository after 60 days without activity. |
| `.github/dependabot.yml`, `.github/workflows/audit.yml` | Dependency hygiene (2026-10-29, audit item 3): Dependabot opens weekly PRs for Go modules (minor/patch grouped) and GitHub Actions; `Audit` runs `govulncheck` v1.9.0 weekly (Monday 06:11 UTC), on demand and on a PR that changes it, with the Go version from `go.mod`. A standard-library finding needs a `go.mod` bump (as in C19), which Dependabot does not make. Not a required check. |
| `.github/workflows/release.yml` | `Release`. On a `v*` tag: `gofmt` check, `go vet`, `go test`, then static (`CGO_ENABLED=0`, `-trimpath -ldflags "-s -w"`) cross-builds on one Linux runner for six platforms: `budget-core-linux-amd64`, `-linux-arm64`, `-darwin-amd64`, `-darwin-arm64`, `-windows-amd64.exe`, `-windows-arm64.exe` (C20; before it, Linux x86-64 only), and `SHA256SUMS` over all of them. A `smoke-test` matrix then runs each binary on a runner of its own platform (`ubuntu-24.04`, `ubuntu-24.04-arm`, `macos-26-intel`, `macos-latest`, `windows-latest`, `windows-11-arm`): it checks the binary's line in `SHA256SUMS` and runs `--help` (a comparison would need testnet for caps). `publish` needs both jobs and runs `gh release create --verify-tag --generate-notes` with every binary and `SHA256SUMS` (`contents: write` only there). On a PR to `main` that changes the workflow, build and smoke tests run and publish is skipped. Uses `checkout@v7`, `setup-go@v7`, `upload-artifact@v7`, `download-artifact@v7` (all `node24`). |
| `README.md`, `CONTRIBUTING.md` | Building, running, flags, exit codes; contribution rules. User-facing explanations live on the docs site, which the README links to. `.gitignore` ignores `/budget-core`, the local build output. |

## 3. Known defects

Checked 2026-10-29 against `origin/main` (`6538c03`): no open defects; `govulncheck ./...` clean. "Open PR" means a fix exists but is not merged to `main`.

| ID | Defect | Evidence | Open PR |
|----|--------|----------|---------|

Resolved and removed: C22 (found 2026-10-29 while measuring several contracts in `budgetgate-action-test` PR #12): function names were never checked for uniqueness, and `CompareSnapshots` matches the baseline by name, so two fixture entries with one name were both compared with one baseline entry (v0.2.2: the relay call, also named `bump`, reported `bump` 291,573 → 587,205, +101.39% REGRESSION, and `relay` REMOVED), and a repeated name in a baseline was silently overwritten. Now a fixture or `--new` snapshot with a repeated name exits 2, and a baseline with one warns and compares those functions as new (maintainer's choice, so Action users with such a cached baseline recover on the next default-branch run). Checked on testnet: the same fixture exits 2; that run's v0.2.2 output used as a baseline shows `bump` and `relay` as new, exit 0. Resolved and removed: C20 (found in the live walkthrough, 2026-10-29): releases only contained `budget-core-linux-amd64`. Since #39 the release workflow cross-builds six platforms and runs each on its own runner before publishing; v0.2.2 (2026-10-29, from `94e7f16`, run 40874792917) is the first such release, and the Action ran all six binaries on matching runners (action A23). The documented checksum command became `sha256sum -c --ignore-missing`, since `SHA256SUMS` now lists six files and plain `-c` exits 1 when only one is downloaded. Resolved and removed: C21 (found in the live walkthrough, 2026-10-29): a testnet reset broke the counter check, the test repo and the documented walkthrough silently, and recovery was a manual multi-step process. Now a weekly `Counter check` workflow measures the fixtures and diffs them with the committed snapshots, and `testdata/counter/redeploy.sh` redeploys and rebuilds everything in one command (run for real: fresh contracts, identical numbers). C19 (found 2026-10-29 by `govulncheck` v1.9.0): `go.mod` said `go 1.25`, so CI and releases built with go1.25.14 (v0.2.0 included), and Go 1.25 no longer gets security fixes; 10 standard-library vulnerabilities were reached by core's code (7 `net/http`, `crypto/tls` GO-2026-6607, `crypto/x509` GO-2026-6809, `net/textproto`), 2 more imported, plus `klauspost/compress` v1.17.6 GO-2026-5841 (required, not imported). `go.mod` now says `go 1.26.10` (the first release with all fixes) and `klauspost/compress` is v1.18.7; `govulncheck ./...` reports no vulnerabilities. The counter contract re-measured on testnet with the go1.26.10 build gives the committed numbers exactly. The v0.2.0 binary keeps the old toolchain; `v0.2.1` (2026-10-29) is the fixed release. C18 (found 2026-10-28): core had no `SECURITY.md`, and the copy in action and dashboard named `security@budgetgate.io`, a domain that does not exist (NXDOMAIN on Google DNS), so reports could not arrive and whoever registered it would receive them. `SECURITY.md` now points to GitHub private vulnerability reporting (maintainer's decision), which the maintainer must enable in the repo settings; it was off on 2026-10-28. The old `--fixture` mock (removed before #4 in `a79bcd9`), Node-based XDR decoding and the always-0 memory (#4), the README/go.mod Go version mismatch (#4), padded CPU instructions (C2b, #5), `read_bytes` blind to contract storage (C2c, #6: footprint entry counts), no HTTP timeout (C8, #8), test data in Go constants and placeholder sample snapshots (C11: real captures and a real snapshot under `testdata/`, table-driven tests), CRLF checkouts on Windows breaking `gofmt -l .` (C15: `.gitattributes`), errors exiting 1 like a regression (C4: every error now exits 2), messages mixed into the report on stdout (C9: stdout is the report only), `--output-json` failures ignored (C13: now exit 2 with the error on stderr), RPC HTTP status not checked (C16: non-200 now fails with the status and a body snippet), added and removed functions skipped silently (C5b: `🆕 NEW` rows with values and cap usage, one `🗑️ REMOVED` row; neither fails the run), one threshold flag only (C3: `--cap-warning-pct` and per-metric absolute-increase flags; a single absolute number across metrics was rejected because CPU and entry counts differ by five orders of magnitude), unversioned schema (C10: snapshots are written as `2.0`; `1.0` and `2.0` are read; anything else exits 2), out-of-date repo docs (C14: `DOCUMENTATION.md` removed in favour of the docs site; README claims checked against the code; `CONTRIBUTING.md` rewritten for Go 1.25 and this repo's rules), a broken committed binary, a stale `PR_BODY.md` and an unexplained `website/` template with a fabricated demo result (C6: all removed, `/budget-core` ignored), placeholder snapshot metadata (C5c: `network` from the RPC's passphrase, `generated_at` in UTC, `commit_sha` from `--commit-sha`), no release (C12: the release workflow in #26, then `v0.2.0` from `61a7a09` on 2026-10-27 with `budget-core-linux-amd64` and `SHA256SUMS`), hardcoded unsourced caps (C5: read live from the network on every run; the old 100M / 10 MB / 1 MB values were 4×–50× off testnet's protocol-29 limits; `--fail-on-cap-warning` opt-in; `pct_of_network_cap` populated). #5–#7 reached `main` through #9; #8 merged into `feat/drop-memory-from-report` before #9 merged and reached `main` with it.

Measurement verified on 2026-10-26 against a deployed counter contract v1 → v2, with core built from #9's head + #8 (= `main`): CPU 291,573 → 449,005, WriteBytes 80 → 380, ReadEntries 3 → 6, WriteEntries 1 → 4, exit 1 on v2; rerun steps in `testdata/counter/README.md`. Re-measured 2026-10-27 with core from `main` (`ce0c23b`): same numbers. Not verified: that the post-#5 CPU figure equals what an executed transaction is charged.

## 4. Work order for this repo

No open defects. Releases: push a `v*` tag only when the maintainer asks, after rerunning the counter-contract check (`testdata/counter/README.md`); `release.yml` does the rest. `v0.2.2` was released on 2026-10-29 from `94e7f16` (C20, C21; six platform binaries, each checksum-verified and run on its own runner before publishing; the published assets pass `sha256sum -c`, and `govulncheck -mode=binary` finds nothing in the Linux, macOS arm64 and Windows x64 binaries). `v0.2.1` was released on 2026-10-29 from `f1e878c` (C19: built with go1.26.10; release run 40851244102; the published binary passes `sha256sum -c`, reports go1.26.10, and `govulncheck -mode=binary` finds nothing). `v0.2.0` was released on 2026-10-27 from `61a7a09`; it was built with go1.25.14 and carries C19's vulnerabilities.

`roadmap.md` tracks progress against this list.

Real measurement (C2b, C2c) is merged and verified, so Drips Wave issues for this repo can now be considered. Write them only when asked (workspace `CLAUDE.md` section 7).

## 5. Commands

```
go build -o budget-core .
go vet ./...
gofmt -l .          # must print nothing
go test ./... -v
./budget-core --baseline testdata/snapshot_testnet.json --new testdata/snapshot_testnet.json
./budget-core --baseline testdata/snapshot_testnet.json --fixture testdata/fixture.json   # live: calls soroban-testnet
```
Run on 2026-10-29 on `6538c03` with go1.26.10: `gofmt -l .` prints nothing, `go vet` clean, `go test -v` passes (21 test functions, 135 passing tests and subtests; `TestRunExitCodes` covers each exit path and checks stdout is empty on errors and starts with the table otherwise); both `budget-core` runs print 10 rows at 0.00% and exit 0. `.gitattributes` keeps text files LF on every OS, so `gofmt -l .` works on Windows too. A clone made before it existed keeps CRLF files until they are checked out again (delete them and `git checkout -- <files>`). `go build -o budget-core .` writes an ignored file; never commit a binary. On a Go older than 1.26.10, the default `GOTOOLCHAIN=auto` downloads go1.26.10; Go 1.22 could not resolve the old `go 1.25` line (no toolchain is named `go1.25`). Check dependencies with `govulncheck ./...` (golang.org/x/vuln v1.9.0 needs Go 1.26 or later to build).

First run against a fresh contract, with no baseline yet:
```
echo '{"schema_version":"2.0","functions":[]}' > empty.json
./budget-core --baseline empty.json --fixture testdata/fixture.json --output-json baseline.json
```

## 6. Rules

### Data integrity
1. No mock, dummy or fallback numbers in any reachable path. On RPC failure, print the error to stderr and exit non-zero.
2. Fixtures live under `testdata/` and are named as fixtures.
3. A measurement feature works only after it has been run against a real testnet contract: change the contract so its cost should move, redeploy, rerun, confirm the numbers moved, and paste before and after in the report. If you cannot run it, say so and mark the work unverified.
4. Do not invent RPC field names, response shapes or network limits. Capture a real `simulateTransaction` response first and save it as test data. Network caps change between protocol versions: read them at implementation time, record the protocol version, never hardcode from memory.

### Schema is a contract
- `SCHEMA.md` is authoritative. Any schema change updates `SCHEMA.md`, bumps `schema_version`, and states the impact on action and dashboard in the PR description.
- Consumers must be able to detect an incompatible `schema_version`.

### Go conventions
- stdout carries the report only. Errors and progress go to stderr.
- Exit codes (implemented, documented in README): 0 pass, 1 regression, 2 usage or runtime error. A new error path returns `exitError`, never `exitRegression`.
- Wrap errors with `%w`. No `panic` outside `main`. No global mutable state.
- HTTP uses an `http.Client` with an explicit timeout and a context. Thresholds come from flags, not constants.
- Table-driven tests. Parser tests use captured real responses.

### Commit and PR conventions
These are the maintainer's rules and override default tool attribution.
- Never add "Co-Authored-By" lines to git commits.
- Do not include Claude attribution or footers in commit messages or pull request descriptions.
- Keep commit messages concise, starting with an imperative verb (e.g., "Fix bug", "Add feature").

### Git
- Never `git add .` or `git add -A`. Stage named files only.
- One logical unit per commit. Work on a branch, push after every commit, open a PR. No pushes to `main`, no force-push.
- Do not commit the compiled binary. Do not create tags or releases unless asked.
- Do not change repo settings or branch protection. Report what needs changing.

### Code style and behavior
- Write clean, idiomatic code matching the existing style of the repository.
- Avoid introducing speculative features or unused dependencies.
- Do not add conversational filler (e.g., "Sure, I can help with that!") inside code comments.
- Fix what was asked. List nearby problems in your report instead of fixing them in the same PR.

## 7. Done means

- [ ] `gofmt -l .` prints nothing, `go vet ./...` is clean, `go test ./... -v` passes
- [ ] If measurement code changed: before and after numbers from a real testnet contract are in the report
- [ ] README, `SCHEMA.md` and flag defaults match the code
- [ ] The report lists what you ran, what you saw, and what you did not verify

## 8. Keeping this file honest

When you fix a defect, delete its row in the same PR. When you find a new one, add a row with evidence. The workspace `CLAUDE.md` carries a copy of these rows. This file is authoritative for this repo. Update the workspace copy when you can.
