# CLAUDE.md — soroban-budget-core

Scope: this repo only. If the workspace-level `CLAUDE.md` exists in the parent folder, read it as well. Where the two differ, this file wins for this repo. The defect table below was checked against `origin/main` (`72f09f9`) on 2026-10-26. Re-verify an item before relying on it.

## 1. What this repo is

The engine of Budget Gate. A Go CLI that takes a baseline snapshot of Soroban contract resource costs and a new snapshot, compares them, and prints a Markdown report. Its second job is to produce the new snapshot itself by calling the Soroban RPC method `simulateTransaction` (`--fixture`). That path makes real RPC calls and was verified against a deployed contract (section 3).

User docs live in the separate `BudgetGate/docs` site (budgetgate.github.io/docs/), which describes this repo as of `72f09f9`. A PR here that changes flags, exit codes, output or the schema needs a matching docs PR, linked from both descriptions.

This repo owns the snapshot schema. The Action and the dashboard both depend on it. It must work standalone, outside GitHub.

Boundaries:
- No GitHub API code, no reference to `soroban-budget-action` or `soroban-budget-dashboard`.
- The only network calls allowed are to a Soroban RPC endpoint.
- Not a static analyzer, not a security scanner. Scout-Soroban already covers that.

## 2. Known layout

State of `main` on 2026-10-26 (`72f09f9`, after #4–#10). Module `github.com/budgetgate/soroban-budget-core`, `go 1.25`. Direct dependencies: `github.com/stellar/go-stellar-sdk v0.7.3` (XDR decoding in `rpc.go`, `txnbuild` in `cmd/genfixture`), `github.com/stretchr/testify v1.11.1`.

| File | Role (read in full unless stated) |
|------|------|
| `main.go` | Flags `--baseline` (required), `--new`, `--fixture` (wins over `--new`), `--rpc-url` (default `https://soroban-testnet.stellar.org`), `--output-json`, `--max-delta-pct` (default 10). With `--fixture` it calls `MeasureResources` per entry and builds the snapshot; with `--new` it reads one. Exit codes 0 pass, 1 regression, 2 usage or runtime error (`exitOK`, `exitRegression`, `exitError`). Logic lives in `run(args, stdout, stderr) int`, so tests call it directly; `main` only calls `os.Exit`. |
| `rpc.go` | `MeasureResources(ctx, rpcURL, txXDR)`: simulates twice with `instructionLeeway` 1,000,000 and 2,000,000, checks the declared values differ by exactly 1,000,000, and reports actual CPU = first − 1,000,000. `simulateTransaction` uses an `http.Client` with a 30 s timeout (`rpcTimeout`) and `NewRequestWithContext`. `ParseTransactionData` decodes `transactionData` into `Resources` (instructions, disk read bytes, write bytes, `ReadEntries`, `WriteEntries`). Fails on `result.error` or missing `transactionData`. `SimulateTransaction` is kept for other callers. |
| `diff.go` | `Thresholds`, `DiffResult`, `getCap` (hardcoded caps), `CompareSnapshots` (CPU, read bytes, write bytes, read entries, write entries; memory not compared), `compareOptionalMetric`, `compareMetric`, `FormatDiffs`. |
| `snapshot.go` | `FixtureEntry`, `Snapshot`, `FunctionCost` (`MemoryBytes`, `ReadEntries`, `WriteEntries` are `*int64`), `ParseSnapshot` (rejects a missing or unknown `schema_version` with `ErrUnsupportedSchema`), `CurrentSchemaVersion` (`"2.0"`). |
| `cmd/genfixture/main.go` | Regenerates `testdata/fixture.json` against testnet: funds an account via friendbot, builds signed `balance` and `transfer` calls on the native-asset contract. `go run ./cmd/genfixture > testdata/fixture.json`. |
| `*_test.go` | testify, table-driven: 9 test functions, 39 cases. Parser and RPC tests read full `simulateTransaction` responses captured from testnet (`testdata/simulate_*.json`); fake RPC servers cover the two-leeway logic and error paths. `TestTestdataSamples` keeps the sample files readable. |
| `SCHEMA.md` | Snapshot format (v2.0, with a version history and the reading rules) and fixture format (`[{name, fixture_id, xdr}]`). Authoritative. |
| `testdata/` | `fixture.json` (`balance_check` read-only, `transfer_to_contract` writes storage; testnet native-asset contract), `simulate_*.json` (captured responses), `snapshot_testnet.json` (real snapshot, schema 2.0). Provenance and regeneration in `testdata/README.md`. |
| `budget-core` | Committed compiled binary, built before #4 (C6). |
| `.github/workflows/ci.yml` | `Core CI`, job `build-and-test`: `go build`, `go test`. Go version from `go.mod`. Runs on push/PR to `main` only. |
| `website/`, `DOCUMENTATION.md`, `PR_BODY.md`, `CONTRIBUTING.md`, `README.md` | See C6 and C14. |

## 3. Known defects

Checked 2026-10-26 against `origin/main` (`72f09f9`). "Open PR" means a fix exists but is not merged to `main`.

| ID | Defect | Evidence | Open PR |
|----|--------|----------|---------|
| C3 | Only one threshold flag (`--max-delta-pct`). Cap-warning threshold (80%) is a constant; `Thresholds.MaxAbsolute` has no flag. | `diff.go` | — |
| C5 | Caps are hardcoded with no source or protocol version (10 MB read / 1 MB write look far above real per-tx limits). A cap warning never fails the run. `pct_of_network_cap` is never populated. | `diff.go` `getCap` | — |
| C5b | Functions only in the new snapshot, or only in the baseline, are silently skipped. | `diff.go` `CompareSnapshots` | — |
| C5c | Snapshot `network` is hardcoded `"testnet"`, `commit_sha` is `"dynamic"`, `generated_at` is never set. | `main.go` | — |
| C6 | Compiled `budget-core` binary committed (pre-#4 build, still calls Node, so broken). `PR_BODY.md` stale. `website/` unexplained. | `git ls-tree origin/main` | — |
| C9 | Progress text and errors go to stdout, mixed with the report. | `main.go` `fmt.Printf` | — |
| C12 | No GitHub release. Only tag `v0.1.0`. The Action downloads `budget-core-linux-amd64` from a release that does not exist. | GitHub API | — |
| C13 | `--output-json` write failures are silent: a `json.MarshalIndent` error skips the write without a message, and the `os.WriteFile` error is discarded (`_ =`). The Action then reads a missing or stale `new.json`. | `main.go:116-121` | — |
| C14 | Repo docs are out of date and duplicate the docs site. `DOCUMENTATION.md` says the default threshold is 5% (it is 10) and calls the CLI "high-performance". `README.md` still says the CLI fails on a "hardcoded network absolute cap" (a cap only warns, C5). `CONTRIBUTING.md` says Go 1.21+ (`go.mod` needs 1.25). | `DOCUMENTATION.md`, `README.md`, `CONTRIBUTING.md`, `go.mod` | — |
| C16 | `simulateTransaction` does not check the HTTP status. A non-200 reply only fails if its body is not valid JSON (\"failed to decode response\"); a 403 HTML page from a proxy gives that same unhelpful message. | `rpc.go` (no `resp.StatusCode` check); a Python client got 403 from soroban-testnet on 2026-10-26 | — |

Resolved and removed: the old `--fixture` mock (removed before #4 in `a79bcd9`), Node-based XDR decoding and the always-0 memory (#4), the README/go.mod Go version mismatch (#4), padded CPU instructions (C2b, #5), `read_bytes` blind to contract storage (C2c, #6: footprint entry counts), no HTTP timeout (C8, #8), test data in Go constants and placeholder sample snapshots (C11: real captures and a real snapshot under `testdata/`, table-driven tests), CRLF checkouts on Windows breaking `gofmt -l .` (C15: `.gitattributes`), errors exiting 1 like a regression (C4: every error now exits 2), unversioned schema (C10: snapshots are written as `2.0`; `1.0` and `2.0` are read; anything else exits 2). #5–#7 reached `main` through #9; #8 merged into `feat/drop-memory-from-report` before #9 merged and reached `main` with it.

Measurement verified on 2026-10-26 against a deployed counter contract v1 → v2, with core built from #9's head + #8 (= `main`): CPU 291,573 → 449,005, WriteBytes 80 → 380, ReadEntries 3 → 6, WriteEntries 1 → 4, exit 1 on v2; rerun steps in the workspace repo's `verification/counter-contract/README.md`. Not verified: that the post-#5 CPU figure equals what an executed transaction is charged.

## 4. Work order for this repo

1. C3, C5, C9, C13, C16: threshold flags, caps read from the network with protocol version recorded, clean stdout, report `--output-json` write errors, check the RPC's HTTP status.
2. C5b, C5c, C6, C14: report new/removed functions, real snapshot metadata, remove the binary (add to `.gitignore`), resolve `PR_BODY.md` and `website/`, bring repo docs in line (or point them at the docs site).
3. C12: tag a release with a built binary only when asked.

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
Run on 2026-10-26 after C4: `gofmt -l .` prints nothing, `go vet` clean, `go test` passes (10 functions, 55 cases; `TestRunExitCodes` covers each exit path); both `budget-core` runs print 10 rows at 0.00% and exit 0. `.gitattributes` keeps text files LF on every OS, so `gofmt -l .` works on Windows too. A clone made before it existed keeps CRLF files until they are checked out again (delete them and `git checkout -- <files>`). `go build -o budget-core .` overwrites the committed binary (C6); do not commit the result. On Go older than 1.25, set `GOTOOLCHAIN=go1.25.0`.

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
