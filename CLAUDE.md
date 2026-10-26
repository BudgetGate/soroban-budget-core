# CLAUDE.md — soroban-budget-core

Scope: this repo only. If the workspace-level `CLAUDE.md` exists in the parent folder, read it as well. Where the two differ, this file wins for this repo. The defect table below was checked against `origin/main` on 2026-10-26. Re-verify an item before relying on it.

## 1. What this repo is

The engine of Budget Gate. A Go CLI that takes a baseline snapshot of Soroban contract resource costs and a new snapshot, compares them, and prints a Markdown report. Its second job is to produce the new snapshot itself by calling the Soroban RPC method `simulateTransaction` (`--fixture`). That path makes real RPC calls; see section 3 for its open defects.

This repo owns the snapshot schema. The Action and the dashboard both depend on it. It must work standalone, outside GitHub.

Boundaries:
- No GitHub API code, no reference to `soroban-budget-action` or `soroban-budget-dashboard`.
- The only network calls allowed are to a Soroban RPC endpoint.
- Not a static analyzer, not a security scanner. Scout-Soroban already covers that.

## 2. Known layout

State of `main` on 2026-10-26 (after PR #4). Module `github.com/budgetgate/soroban-budget-core`, `go 1.25`. Direct dependencies: `github.com/stellar/go-stellar-sdk v0.7.3` (imported in `rpc.go` for XDR decoding), `github.com/stretchr/testify v1.11.1`.

| File | Role (read in full unless stated) |
|------|------|
| `main.go` | Flags `--baseline` (required), `--new`, `--fixture`, `--rpc-url` (default `https://soroban-testnet.stellar.org`), `--output-json`, `--max-delta-pct` (default 10). With `--fixture` it simulates each fixture entry and builds the new snapshot; with `--new` it reads one. Exits 1 on a regression and on every error. |
| `rpc.go` | `SimulateTransaction(rpcURL, txXDR)`, `ParseTransactionData` (decodes `transactionData` XDR into `Resources`: instructions, disk read bytes, write bytes), `parseOptionalInt`. Fails on `result.error` or missing `transactionData`. |
| `diff.go` | `Thresholds`, `DiffResult`, `getCap` (hardcoded caps), `CompareSnapshots`, `compareOptionalMetric`, `compareMetric`, `FormatDiffs`. |
| `snapshot.go` | `FixtureEntry`, `Snapshot`, `FunctionCost` (`MemoryBytes` is `*int64`), `ParseSnapshot`. |
| `*_test.go` | testify tests for diff, RPC (captured testnet `transactionData` as string constants) and snapshot parsing. |
| `SCHEMA.md` | Snapshot format (v1.0) and fixture format (`[{name, fixture_id, xdr}]`). Authoritative. |
| `baseline.json`, `new.json` | Sample snapshots with placeholder numbers, not measurements. |
| `fixture.json` | One `balance_check` fixture against the testnet native-asset contract. |
| `budget-core` | Committed compiled binary, built before #4 (C6). |
| `.github/workflows/ci.yml` | `Core CI`, job `build-and-test`: `go build`, `go test`. Go version from `go.mod`. Runs on push/PR to `main` only. |
| `website/`, `DOCUMENTATION.md`, `PR_BODY.md`, `CONTRIBUTING.md` | `PR_BODY.md` is a stale PR description. `website/` is an unexplained Vite app. The others are not reviewed. |

Pending, not on `main` yet: PR #9 lands the reviewed #5–#7 (exact CPU via two-leeway simulation, `read_entries`/`write_entries`, `cmd/genfixture`, `transfer_to_contract` fixture, memory dropped from the report). PR #8 adds the HTTP timeout and must be retargeted from `feat/drop-memory-from-report` to `main` before merging.

## 3. Known defects

Checked 2026-10-26 against `origin/main`. "Open PR" means a fix exists but is not merged to `main`.

| ID | Defect | Evidence | Open PR |
|----|--------|----------|---------|
| C2b | CPU instructions are the RPC's declared value: actual + max(instructionLeeway, provider pad; 50,000 on testnet). Understates % deltas. | Probe: leeway 0/1/50k → 141,406; 100k → 191,406; 1M → 1,091,406 | #9 |
| C2c | `diskReadBytes` excludes live contract-storage reads (protocol 23), so `read_bytes` cannot catch added storage access. Only the fixture is read-only. | getLedgerEntries sizes vs diskReadBytes (144) | #9 |
| C3 | Only one threshold flag (`--max-delta-pct`). Cap-warning threshold (80%) is a constant; `Thresholds.MaxAbsolute` has no flag. | `diff.go` | — |
| C4 | Usage and runtime errors exit 1, same as a regression. No exit code 2. | `main.go` | — |
| C5 | Caps are hardcoded with no source or protocol version (10 MB read / 1 MB write look far above real per-tx limits). A cap warning never fails the run. `pct_of_network_cap` is never populated. | `diff.go` `getCap` | — |
| C5b | Functions only in the new snapshot, or only in the baseline, are silently skipped. | `diff.go` `CompareSnapshots` | — |
| C5c | Snapshot `network` is hardcoded `"testnet"`, `commit_sha` is `"dynamic"`, `generated_at` is never set. | `main.go` | — |
| C6 | Compiled `budget-core` binary committed (pre-#4 build, still calls Node, so broken). `PR_BODY.md` stale. `website/` unexplained. | `git ls-tree origin/main` | — |
| C8 | `rpc.go` uses `http.Post`: no timeout, no context. | `rpc.go` | #8 |
| C9 | Progress text and errors go to stdout, mixed with the report. | `main.go` `fmt.Printf` | — |
| C10 | `memory_bytes` became nullable in #4 (and #9 adds `read_entries`/`write_entries`) without bumping `schema_version`. Core never checks a baseline's `schema_version`. | `SCHEMA.md`, `snapshot.go` | — |
| C11 | Captured RPC responses are Go string constants, not files under `testdata/`. Tests are not table-driven. `baseline.json`, `new.json`, `fixture.json` sit in the repo root; `baseline.json` holds placeholder numbers. `baseline.json` (`balance_check`) and `new.json` (`mint`) share no function, so the README example `--baseline baseline.json --new new.json` prints an empty table and exits 0. | `rpc_test.go`, root listing, run 2026-10-26 | — |
| C12 | No GitHub release. Only tag `v0.1.0`. The Action downloads `budget-core-linux-amd64` from a release that does not exist. | GitHub API | — |

Resolved and removed: the old `--fixture` mock (removed before #4 in `a79bcd9`), Node-based XDR decoding and the always-0 memory (#4), the README/go.mod Go version mismatch (#4). Measurement was verified on 2026-10-26 against a deployed counter contract v1 → v2 (numbers in PR #4). Not verified: that the post-#5 CPU figure equals what an executed transaction is charged.

## 4. Work order for this repo

1. Land #9, then retarget and land #8 (C2b, C2c, C8).
2. C10, C11: bump the schema version and validate it on load; move captured responses to `testdata/`, table-driven tests.
3. C3, C4, C5, C9: threshold flags, exit codes 0/1/2, caps read from the network with protocol version recorded, clean stdout.
4. C5b, C5c, C6: report new/removed functions, real snapshot metadata, remove the binary (add to `.gitignore`), resolve `PR_BODY.md` and `website/`.
5. C12: tag a release with a built binary only when asked.

`roadmap.md` tracks progress against this list.

Drips Wave issues for this repo can be considered once step 1 has landed.

## 5. Commands

```
go build -o budget-core .
go vet ./...
gofmt -l .          # must print nothing
go test ./... -v
./budget-core --baseline baseline.json --new new.json
./budget-core --baseline baseline.json --fixture fixture.json   # live: calls soroban-testnet
```
`go vet`, `gofmt -l .`, `go test` and both `budget-core` runs were run on `main` on 2026-10-26: the `--new` run prints an empty table (C11), the `--fixture` run reports a CPU "regression" against the placeholder baseline and exits 1. `go build -o budget-core .` overwrites the committed binary (C6); do not commit the result. On Go older than 1.25, set `GOTOOLCHAIN=go1.25.0`.

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
- Proposed exit codes, to be documented in the README when implemented: 0 pass, 1 regression, 2 usage or runtime error.
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
