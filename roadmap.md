# soroban-budget-core roadmap

Progress tracker for this repo. The defect IDs (C2b, C3, …) and their evidence are in `CLAUDE.md` section 3; the order comes from `CLAUDE.md` section 4. Tick a box only when the change is merged to `main`, not when a PR is open.

Last updated: 2026-10-26 (after #9)

## Where I left off

- Phase 1 is done. `main` (`72f09f9`) has #4–#10: native XDR decoding, actual CPU via two-leeway simulation, footprint entry counts, memory out of the comparison, HTTP timeout with context. Verified against the deployed counter contract (numbers in `CLAUDE.md` section 3).
- The docs site (budgetgate.github.io/docs/) now describes this `main`, including its known limitations. Changing user-facing behaviour needs a matching `BudgetGate/docs` PR.
- C10 done: snapshots are written as schema `2.0`; `1.0` and `2.0` are read; a missing or unknown version exits 2 with an error on stderr.
- C11 done: test data lives in `testdata/` (captured testnet responses, the fixture, a real snapshot); tests are table-driven; the README example prints a full report.
- C15 done: `.gitattributes` keeps text files LF, so `gofmt -l .` works on Windows.
- Phase 2 is done except committing the counter contract.
- C4 done: exit codes 0 pass, 1 regression, 2 error; `run()` is unit-tested per exit path. The Action's A8 is unblocked.
- C9 done: stdout carries only the Markdown report; usage, progress and errors go to stderr.
- C13 done: a `--output-json` file that cannot be written fails the run (exit 2, error on stderr); the report is still printed.
- C16 done: a non-200 RPC reply fails with `rpc returned HTTP <code>: <body snippet>` instead of a JSON decode error.
- C3 done: `--cap-warning-pct` and per-metric absolute-increase flags (`--max-cpu-increase`, …); negative thresholds exit 2.
- C5b done: added functions get `🆕 NEW` rows (values and cap usage), removed functions one `🗑️ REMOVED` row; neither fails the run.
- Next up: C5 (network caps). Decided 2026-10-27: read caps live from the RPC each run (exit 2 if the lookup fails), skip it when `--cap-warning-pct 0`, warnings stay non-failing unless `--fail-on-cap-warning`. Testnet limits captured 2026-10-26 (protocol 29): CPU 400,000,000; disk read bytes 200,000; write bytes 132,096; write entries 200; footprint entries 400. Then C5c, C6, C14.
- Rule learned the hard way: do not stack PRs on feature branches. Base every PR on `main`.

## Phase 1 — Real measurement

- [x] Decode `transactionData` XDR in Go, drop the Node script and committed `node_modules/` (#4)
- [x] Record `memory_bytes` as `null` instead of a fake 0; fail on `result.error` (#4)
- [x] C2b: report actual CPU instructions, not the RPC's padded estimate (#5, via #9)
- [x] C2c: footprint entry counts, storage-writing fixture, `cmd/genfixture` (#6, via #9)
- [x] Drop memory from the comparison until a real source exists (#7, via #9)
- [x] C8: `http.Client` with timeout and context for all HTTP (#8, via #9)
- [x] Verify against a deployed contract v1 → v2 with the merged code; numbers moved as expected (2026-10-26)

## Phase 2 — Schema and test data

- [x] C10: bump `schema_version` to `2.0`, document it in `SCHEMA.md`, reject snapshots with a missing or unknown version (exit 2)
- [x] C11: move captured RPC responses to `testdata/`, table-driven tests, move sample snapshots and fixtures out of the repo root, make the README example produce a non-empty report
- [x] C15: `.gitattributes` so Go files stay LF on Windows checkouts
- [ ] Commit the v1/v2 counter contract used for verification under `testdata/` so the check can be rerun (it lives in the private workspace repo today)

## Phase 3 — Thresholds, exit codes, output

- [x] C3: flags for the cap-warning threshold and per-metric absolute-increase thresholds
- [x] C4: exit codes 0 pass, 1 regression, 2 usage/runtime error; document in README
- [ ] C5: read network caps from the network, record the protocol version, decide whether a cap breach fails the run, populate `pct_of_network_cap`
- [x] C9: report on stdout only; progress and errors on stderr
- [x] C13: fail with a clear error when `--output-json` cannot be written
- [x] C16: check the RPC's HTTP status and report it

## Phase 4 — Correct reports and a clean repo

- [x] C5b: report functions added to or removed from the baseline
- [ ] C5c: real `network`, `commit_sha` and `generated_at` in snapshots
- [ ] C6: remove the committed `budget-core` binary and add it to `.gitignore`; resolve `PR_BODY.md` and `website/`
- [ ] C14: README, `DOCUMENTATION.md` and `CONTRIBUTING.md` match the code, or point to the docs site
- [x] Replace placeholder `baseline.json`/`new.json` with a real snapshot (`testdata/snapshot_testnet.json`, C11)

## Phase 5 — Release (only when the maintainer asks)

- [ ] C12: GitHub release with `budget-core-linux-amd64` (the name the Action downloads)

## Maintainer actions (Claude must not do these)

- [ ] Branch protection on `main`: currently 0 required reviews and no required status checks. Consider requiring `Core CI / build-and-test`.
- [ ] Repo topics and description (none set).
- [ ] Delete stale branches (checked 2026-10-26):
  - Fully in `main`: `feat/drop-memory-from-report`, `fix/http-client-timeout`, `docs/claude-md-and-roadmap`, `feature/complete-missing-specs`.
  - One GitHub merge commit ahead of `main`, but both its parents are in `main`, so no unique content: `fix/native-xdr-decoding`, `fix/exact-cpu-instructions`, `feat/storage-fixtures-footprint`.
  - Has one unmerged commit (`3fedc4f`): `feature/ui-ux-docs-overhaul` adds `SECURITY.md` (the missing core copy; same text as action/dashboard), `pr_body_core.md`, and a 7.6 MB compiled binary `soroban-budget-core`. Do not merge it. Salvage `SECURITY.md` in its own PR if wanted, then delete the branch.
