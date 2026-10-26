# Contributing to soroban-budget-core

## Local setup

1. Install Go 1.25 or later (`go.mod` requires it).
2. Clone the repository and run `go mod download`.
3. Check your environment:
   ```bash
   gofmt -l .        # must print nothing
   go vet ./...
   go test ./...
   ```
   The tests need no network: they use responses captured from soroban-testnet, stored in `testdata/`.

Line endings are fixed to LF by `.gitattributes`, so `gofmt -l .` works on Windows too. If you cloned before that file existed, delete the Go files and run `git checkout -- .` to refresh them.

## Rules for changes

- **No made-up numbers.** If an RPC call fails, print the error to stderr and exit `2`. Never fall back to fixed or estimated values.
- **Capture before you parse.** Code that reads a new RPC response needs a real response captured with `curl` and saved under `testdata/`, with its provenance in `testdata/README.md`.
- **Measurement changes need a testnet run.** If you change how a cost is measured, run core against a real testnet contract, change the contract so the cost should move, and put the before and after numbers in the PR.
- **Table-driven tests** with testify.
- **Output streams:** stdout carries only the Markdown report. Progress and errors go to stderr.
- **Exit codes:** `0` pass, `1` regression, `2` any usage or runtime error. A new error path returns `2`.
- **Snapshot schema:** `SCHEMA.md` is the contract with the Action and the dashboard. Changing a field's name, type or meaning bumps `schema_version` and needs a note on the impact in the PR.
- **Never commit a binary.** `/budget-core` is ignored.

## Submitting a PR

- Base your branch on `main`.
- Link the issue it resolves (`Closes #<n>`), and keep the change to that issue.
- Say in the PR what you ran and what you saw.
- If you change flags, output, exit codes or the schema, open a matching PR in [BudgetGate/docs](https://github.com/BudgetGate/docs) and link the two.
