package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/stellar/go-stellar-sdk/network"
	"github.com/stellar/go-stellar-sdk/xdr"
)

// NetworkCaps are the network's per-transaction resource limits, read from its config settings
// so they follow protocol upgrades and differ correctly between networks.
type NetworkCaps struct {
	ProtocolVersion int
	Passphrase      string           // network passphrase reported by getNetwork
	Ledger          int64            // latest ledger when the limits were read
	Limits          map[string]int64 // by metric name, e.g. "CPUInstructions"
}

// networkName returns the usual name for a known network passphrase, or the passphrase itself.
func networkName(passphrase string) string {
	switch passphrase {
	case network.PublicNetworkPassphrase:
		return "mainnet"
	case network.TestNetworkPassphrase:
		return "testnet"
	case network.FutureNetworkPassphrase:
		return "futurenet"
	}
	return passphrase
}

// capSettings are the config settings that hold the per-transaction limits core compares against.
func capSettings() []xdr.ConfigSettingId {
	return []xdr.ConfigSettingId{
		xdr.ConfigSettingIdConfigSettingContractComputeV0,
		xdr.ConfigSettingIdConfigSettingContractLedgerCostV0,
		xdr.ConfigSettingIdConfigSettingContractLedgerCostExtV0,
	}
}

// FetchNetworkCaps reads the per-transaction limits and the protocol version from the RPC.
func FetchNetworkCaps(ctx context.Context, rpcURL string) (NetworkCaps, error) {
	var keys []string
	for _, id := range capSettings() {
		k, err := xdr.MarshalBase64(xdr.LedgerKey{
			Type:          xdr.LedgerEntryTypeConfigSetting,
			ConfigSetting: &xdr.LedgerKeyConfigSetting{ConfigSettingId: id},
		})
		if err != nil {
			return NetworkCaps{}, fmt.Errorf("encode config setting key: %w", err)
		}
		keys = append(keys, k)
	}

	body, err := postJSONRPC(ctx, rpcURL, "getLedgerEntries", map[string]interface{}{"keys": keys})
	if err != nil {
		return NetworkCaps{}, fmt.Errorf("getLedgerEntries: %w", err)
	}
	limits, ledger, err := parseCapEntries(body)
	if err != nil {
		return NetworkCaps{}, err
	}

	body, err = postJSONRPC(ctx, rpcURL, "getNetwork", nil)
	if err != nil {
		return NetworkCaps{}, fmt.Errorf("getNetwork: %w", err)
	}
	protocol, passphrase, err := parseNetworkInfo(body)
	if err != nil {
		return NetworkCaps{}, err
	}

	return NetworkCaps{ProtocolVersion: protocol, Passphrase: passphrase, Ledger: ledger, Limits: limits}, nil
}

// parseCapEntries maps a getLedgerEntries response for capSettings to limits by metric name.
//
// read_entries counts every footprint entry, so its limit is TxMaxFootprintEntries, not
// TxMaxDiskReadEntries; read_bytes is disk read bytes, so its limit is TxMaxDiskReadBytes.
func parseCapEntries(body []byte) (map[string]int64, int64, error) {
	var resp struct {
		Result struct {
			Entries []struct {
				XDR string `json:"xdr"`
			} `json:"entries"`
			LatestLedger int64 `json:"latestLedger"`
		} `json:"result"`
		Error *rpcError `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, 0, fmt.Errorf("getLedgerEntries: failed to decode response: %w", err)
	}
	if resp.Error != nil {
		return nil, 0, fmt.Errorf("getLedgerEntries: %w", resp.Error)
	}

	limits := make(map[string]int64)
	for _, e := range resp.Result.Entries {
		var data xdr.LedgerEntryData
		if err := xdr.SafeUnmarshalBase64(e.XDR, &data); err != nil {
			return nil, 0, fmt.Errorf("getLedgerEntries: failed to decode entry XDR: %w", err)
		}
		cs, ok := data.GetConfigSetting()
		if !ok {
			return nil, 0, fmt.Errorf("getLedgerEntries: entry is %s, not a config setting", data.Type)
		}
		switch cs.ConfigSettingId {
		case xdr.ConfigSettingIdConfigSettingContractComputeV0:
			limits["CPUInstructions"] = int64(cs.MustContractCompute().TxMaxInstructions)
		case xdr.ConfigSettingIdConfigSettingContractLedgerCostV0:
			c := cs.MustContractLedgerCost()
			limits["ReadBytes"] = int64(c.TxMaxDiskReadBytes)
			limits["WriteBytes"] = int64(c.TxMaxWriteBytes)
			limits["WriteEntries"] = int64(c.TxMaxWriteLedgerEntries)
		case xdr.ConfigSettingIdConfigSettingContractLedgerCostExtV0:
			limits["ReadEntries"] = int64(cs.MustContractLedgerCostExt().TxMaxFootprintEntries)
		}
	}

	for _, metric := range []string{"CPUInstructions", "ReadBytes", "WriteBytes", "ReadEntries", "WriteEntries"} {
		if limits[metric] <= 0 {
			return nil, 0, fmt.Errorf("getLedgerEntries: no limit for %s in the network's config settings", metric)
		}
	}
	return limits, resp.Result.LatestLedger, nil
}

// parseNetworkInfo reads protocolVersion and passphrase from a getNetwork response.
func parseNetworkInfo(body []byte) (int, string, error) {
	var resp struct {
		Result struct {
			ProtocolVersion int    `json:"protocolVersion"`
			Passphrase      string `json:"passphrase"`
		} `json:"result"`
		Error *rpcError `json:"error,omitempty"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return 0, "", fmt.Errorf("getNetwork: failed to decode response: %w", err)
	}
	if resp.Error != nil {
		return 0, "", fmt.Errorf("getNetwork: %w", resp.Error)
	}
	if resp.Result.ProtocolVersion <= 0 {
		return 0, "", fmt.Errorf("getNetwork: response has no protocolVersion")
	}
	if resp.Result.Passphrase == "" {
		return 0, "", fmt.Errorf("getNetwork: response has no passphrase")
	}
	return resp.Result.ProtocolVersion, resp.Result.Passphrase, nil
}
