// Command genfixture regenerates fixture.json against soroban-testnet.
//
// It funds a fresh account with friendbot and builds one transaction envelope per fixture,
// calling the testnet native asset contract:
//
//   - balance_check:        balance(source) — reads only
//   - transfer_to_contract: transfer(source, <contract>, 1 XLM) — writes the source account and
//     creates a contract-data balance entry
//
// Authorization entries are recorded with a simulation in "record" mode and attached to the
// final envelope, so the fixtures simulate cleanly in the default "enforce" mode.
//
// Testnet is reset periodically; rerun this when the fixture account no longer exists:
//
//	go run ./cmd/genfixture > fixture.json
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/stellar/go-stellar-sdk/keypair"
	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/strkey"
	"github.com/stellar/go-stellar-sdk/txnbuild"
	"github.com/stellar/go-stellar-sdk/xdr"
)

const (
	rpcURL       = "https://soroban-testnet.stellar.org"
	friendbotURL = "https://friendbot.stellar.org"
	// Native asset (XLM) contract on testnet.
	nativeContract = "CDLZFC3SYJYDZT7K67VZ75HPJVIEUVNIXF47ZG2FB2RMQQVU2HHGCYSC"
	// httpTimeout bounds each RPC and friendbot round trip.
	httpTimeout = 30 * time.Second
)

type fixture struct {
	Name      string `json:"name"`
	FixtureID string `json:"fixture_id"`
	XDR       string `json:"xdr"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "genfixture:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx := context.Background()

	kp, err := keypair.Random()
	if err != nil {
		return err
	}
	if err := fund(ctx, kp.Address()); err != nil {
		return err
	}

	contractID, err := strkey.Decode(strkey.VersionByteContract, nativeContract)
	if err != nil {
		return err
	}
	var cid xdr.ContractId
	copy(cid[:], contractID)
	contract := xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeContract, ContractId: &cid}

	accountID, err := xdr.AddressToAccountId(kp.Address())
	if err != nil {
		return err
	}
	source := xdr.ScAddress{Type: xdr.ScAddressTypeScAddressTypeAccount, AccountId: &accountID}

	oneXLM := xdr.Int128Parts{Hi: 0, Lo: 10_000_000}
	calls := []struct {
		name, id, fn string
		args         []xdr.ScVal
	}{
		{"balance_check", "test_balance", "balance", []xdr.ScVal{addressVal(source)}},
		{"transfer_to_contract", "test_transfer_to_contract", "transfer", []xdr.ScVal{
			addressVal(source),
			addressVal(contract),
			{Type: xdr.ScValTypeScvI128, I128: &oneXLM},
		}},
	}

	var fixtures []fixture
	for _, c := range calls {
		hostFn := xdr.HostFunction{
			Type: xdr.HostFunctionTypeHostFunctionTypeInvokeContract,
			InvokeContract: &xdr.InvokeContractArgs{
				ContractAddress: contract,
				FunctionName:    xdr.ScSymbol(c.fn),
				Args:            c.args,
			},
		}

		draft, err := buildEnvelope(kp, hostFn, nil)
		if err != nil {
			return err
		}
		auth, err := recordAuth(ctx, draft)
		if err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
		env, err := buildEnvelope(kp, hostFn, auth)
		if err != nil {
			return err
		}
		fixtures = append(fixtures, fixture{Name: c.name, FixtureID: c.id, XDR: env})
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(fixtures)
}

func addressVal(a xdr.ScAddress) xdr.ScVal {
	return xdr.ScVal{Type: xdr.ScValTypeScvAddress, Address: &a}
}

// buildEnvelope returns a signed base64 envelope. simulateTransaction ignores the sequence number.
func buildEnvelope(kp *keypair.Full, hostFn xdr.HostFunction, auth []xdr.SorobanAuthorizationEntry) (string, error) {
	tx, err := txnbuild.NewTransaction(txnbuild.TransactionParams{
		SourceAccount:        &txnbuild.SimpleAccount{AccountID: kp.Address(), Sequence: 1},
		IncrementSequenceNum: true,
		Operations:           []txnbuild.Operation{&txnbuild.InvokeHostFunction{HostFunction: hostFn, Auth: auth}},
		BaseFee:              txnbuild.MinBaseFee,
		Preconditions:        txnbuild.Preconditions{TimeBounds: txnbuild.NewInfiniteTimeout()},
	})
	if err != nil {
		return "", err
	}
	tx, err = tx.Sign(network.TestNetworkPassphrase, kp)
	if err != nil {
		return "", err
	}
	return tx.Base64()
}

// recordAuth simulates in "record" mode and returns the authorization entries the call needs.
func recordAuth(ctx context.Context, envelope string) ([]xdr.SorobanAuthorizationEntry, error) {
	body, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "simulateTransaction",
		"params":  map[string]string{"transaction": envelope, "authMode": "record"},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: httpTimeout}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out struct {
		Result struct {
			Error   string `json:"error"`
			Results []struct {
				Auth []string `json:"auth"`
			} `json:"results"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Error != nil {
		return nil, fmt.Errorf("rpc error: %s", out.Error.Message)
	}
	if out.Result.Error != "" {
		return nil, fmt.Errorf("simulation failed: %s", out.Result.Error)
	}

	var auth []xdr.SorobanAuthorizationEntry
	for _, r := range out.Result.Results {
		for _, a := range r.Auth {
			var entry xdr.SorobanAuthorizationEntry
			if err := xdr.SafeUnmarshalBase64(a, &entry); err != nil {
				return nil, err
			}
			auth = append(auth, entry)
		}
	}
	return auth, nil
}

// fund creates the account on testnet and waits until the RPC can see it.
func fund(ctx context.Context, address string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, friendbotURL+"/?addr="+address, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: httpTimeout}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("friendbot returned %s: %s", resp.Status, b)
	}
	// Friendbot responds once the transaction is in a ledger; give the RPC a ledger to catch up.
	time.Sleep(6 * time.Second)
	return nil
}
