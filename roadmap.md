# soroban-budget-core roadmap

Progress tracker for this repo. The defect IDs (C2b, C3, …) and their evidence are in `CLAUDE.md` section 3; the order comes from `CLAUDE.md` section 4. Tick a box only when the change is merged to `main`, not when a PR is open.

Last updated: 2026-10-26

## Where I left off

- `main` has PR #4: native Go XDR decoding, no Node dependency, nullable `memory_bytes`, failed simulations exit non-zero.
- #5, #6 and #7 were merged into their stacked base branches instead of `main`. **PR #9** lands them on `main` unchanged.
- **PR #8** (HTTP timeout) still targets `feat/drop-memory-from-report`. Retarget it to `main` after #9 merges, then merge it.
- Next up after that: phase 2 (schema version, `testdata/`).
- Rule learned the hard way: do not stack PRs on feature branches. Base every PR on `main`.

## Phase 1 — Real measurement

- [x] Decode `transactionData` XDR in Go, drop the Node script and committed `node_modules/` (#4)
- [x] Record `memory_bytes` as `null` instead of a fake 0; fail on `result.error` (#4)
- [x] Verify against a deployed contract v1 → v2; numbers moved as expected (2026-10-26, evidence in #4)
- [ ] C2b: report actual CPU instructions, not the RPC's padded estimate (#5, landing via #9)
- [ ] C2c: footprint entry counts, storage-writing fixture, `cmd/genfixture` (#6, landing via #9)
- [ ] Drop memory from the comparison until a real source exists (#7, landing via #9)
- [ ] C8: `http.Client` with timeout and context for all HTTP (#8)

## Phase 2 — Schema and test data

- [ ] C10: bump `schema_version`, document v2 in `SCHEMA.md`, reject baselines with an unknown version
- [ ] C11: move captured RPC responses to `testdata/`, table-driven tests, move sample snapshots and fixtures out of the repo root, make the README example produce a non-empty report
- [ ] Commit the v1/v2 counter contract used for verification under `testdata/` so the check can be rerun

## Phase 3 — Thresholds, exit codes, output

- [ ] C3: flags for the cap-warning threshold and the absolute-delta threshold
- [ ] C4: exit codes 0 pass, 1 regression, 2 usage/runtime error; document in README
- [ ] C5: read network caps from the network, record the protocol version, decide whether a cap breach fails the run, populate `pct_of_network_cap`
- [ ] C9: report on stdout only; progress and errors on stderr

## Phase 4 — Correct reports and a clean repo

- [ ] C5b: report functions added to or removed from the baseline
- [ ] C5c: real `network`, `commit_sha` and `generated_at` in snapshots
- [ ] C6: remove the committed `budget-core` binary and add it to `.gitignore`; resolve `PR_BODY.md` and `website/`
- [ ] Regenerate `baseline.json` from real measurements

## Phase 5 — Release (only when the maintainer asks)

- [ ] C12: GitHub release with `budget-core-linux-amd64` (the name the Action downloads)

## Maintainer actions (Claude must not do these)

- [ ] Branch protection on `main`: currently 0 required reviews and no required status checks. Consider requiring `Core CI / build-and-test`.
- [ ] Repo topics and description (none set).
- [ ] Delete the merged intermediate branches `fix/native-xdr-decoding`, `fix/exact-cpu-instructions` and `feat/storage-fixtures-footprint` once #9 is in.
