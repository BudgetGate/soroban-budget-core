# testdata

Files used by the tests and by the README examples. All numbers come from soroban-testnet; none are made up.

| File | What it is | How it was made |
|---|---|---|
| `fixture.json` | Fixture with two calls on the testnet native-asset contract: `balance_check` (read-only) and `transfer_to_contract` (writes storage). | `go run ./cmd/genfixture > testdata/fixture.json` |
| `simulate_balance_check.json` | Full `simulateTransaction` response for `balance_check`, captured 2026-10-26 at ledger 5405465. Default leeway, so `transactionData` declares 141,406 instructions (91,406 actual + the 50,000 testnet pad). | `curl` POST to `https://soroban-testnet.stellar.org`, pretty-printed |
| `simulate_transfer_to_contract.json` | Same for `transfer_to_contract`: 268,663 declared instructions (218,663 actual), 368 write bytes, 3 read entries, 2 write entries. | as above |
| `getledgerentries_config.json` | `getLedgerEntries` response for the three config settings that hold per-transaction limits (`CONTRACT_COMPUTE_V0`, `CONTRACT_LEDGER_COST_V0`, `CONTRACT_LEDGER_COST_EXT_V0`), captured 2026-10-27 at ledger 5406471. Protocol 29 limits: 400,000,000 instructions, 200,000 disk read bytes, 132,096 write bytes, 200 write entries, 400 footprint entries. | `curl` POST, `"params":{"keys":["AAAACAAAAAE=","AAAACAAAAAI=","AAAACAAAAA8="]}` |
| `getnetwork.json` | `getNetwork` response (`protocolVersion` 29). | `curl` POST, `"method":"getNetwork"` |
| `snapshot_testnet.json` | Snapshot written by core (schema `2.0`) from `fixture.json`. `generated_at` is empty and `commit_sha` is `"dynamic"` because core does not fill them yet (C5c). | `budget-core --baseline <empty 2.0 snapshot> --fixture testdata/fixture.json --output-json testdata/snapshot_testnet.json` |

Testnet is reset periodically. After a reset the fixture's source account no longer exists and simulation fails: regenerate `fixture.json`, then recapture the two responses and the snapshot. The resource numbers above should not change, because the native-asset contract is the same on every reset; if they do, the tests will say so.

Capture command (replace `<xdr>` with an entry's `xdr` from `fixture.json`):

```bash
curl -sS -X POST https://soroban-testnet.stellar.org \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"simulateTransaction","params":{"transaction":"<xdr>"}}'
```
