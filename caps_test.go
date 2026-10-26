package main

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testnetCaps returns the per-transaction limits in testdata/getledgerentries_config.json,
// captured from soroban-testnet at protocol 29.
func testnetCaps() map[string]int64 {
	return map[string]int64{
		"CPUInstructions": 400_000_000,
		"ReadBytes":       200_000,
		"WriteBytes":      132_096,
		"ReadEntries":     400,
		"WriteEntries":    200,
	}
}

func TestParseCapEntries(t *testing.T) {
	limits, ledger, err := parseCapEntries(capturedFile(t, "getledgerentries_config.json"))
	require.NoError(t, err)
	assert.Equal(t, testnetCaps(), limits)
	assert.Equal(t, int64(5406471), ledger)
}

func TestParseCapEntries_Errors(t *testing.T) {
	// Drop the footprint-entries setting from the captured response.
	var captured map[string]interface{}
	require.NoError(t, json.Unmarshal(capturedFile(t, "getledgerentries_config.json"), &captured))
	result := captured["result"].(map[string]interface{})
	result["entries"] = result["entries"].([]interface{})[:2]
	missingExt, err := json.Marshal(captured)
	require.NoError(t, err)

	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{name: "setting missing", body: string(missingExt), wantErr: "no limit for ReadEntries"},
		{name: "no entries", body: `{"jsonrpc":"2.0","id":1,"result":{"entries":[],"latestLedger":1}}`, wantErr: "no limit for CPUInstructions"},
		{name: "JSON-RPC error", body: `{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"invalid key"}}`, wantErr: "rpc returned error: -32602 invalid key"},
		{name: "bad entry XDR", body: `{"jsonrpc":"2.0","id":1,"result":{"entries":[{"xdr":"not-xdr"}]}}`, wantErr: "failed to decode entry XDR"},
		{name: "not JSON", body: `<html>`, wantErr: "failed to decode response"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseCapEntries([]byte(tt.body))
			assert.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestParseProtocolVersion(t *testing.T) {
	tests := []struct {
		name    string
		body    func(t *testing.T) []byte
		want    int
		wantErr string
	}{
		{name: "captured getNetwork", body: func(t *testing.T) []byte { return capturedFile(t, "getnetwork.json") }, want: 29},
		{name: "no protocolVersion", body: func(t *testing.T) []byte { return []byte(`{"jsonrpc":"2.0","id":1,"result":{}}`) }, wantErr: "no protocolVersion"},
		{name: "JSON-RPC error", body: func(t *testing.T) []byte {
			return []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"method not found"}}`)
		}, wantErr: "-32601 method not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProtocolVersion(tt.body(t))
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFetchNetworkCaps(t *testing.T) {
	ok := paddingRPC(t, 1000, 50000)
	defer ok.Close()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()

	caps, err := FetchNetworkCaps(context.Background(), ok.URL)
	require.NoError(t, err)
	assert.Equal(t, testnetCaps(), caps.Limits)
	assert.Equal(t, 29, caps.ProtocolVersion)
	assert.Equal(t, int64(5406471), caps.Ledger)

	_, err = FetchNetworkCaps(context.Background(), down.URL)
	assert.ErrorContains(t, err, "getLedgerEntries: rpc returned HTTP 503")
}

func TestPctOfNetworkCap(t *testing.T) {
	tests := []struct {
		name string
		fn   FunctionCost
		caps map[string]int64
		want float64
	}{
		// The real transfer_to_contract costs: write entries (2 of 200) are the closest to their cap.
		{name: "highest share wins", fn: FunctionCost{CPUInstructions: 218663, ReadBytes: 144, WriteBytes: 368, ReadEntries: int64Ptr(3), WriteEntries: int64Ptr(2)}, caps: testnetCaps(), want: 1.0},
		{name: "entry counts missing", fn: FunctionCost{CPUInstructions: 200_000_000, ReadBytes: 144}, caps: testnetCaps(), want: 50},
		{name: "no caps", fn: FunctionCost{CPUInstructions: 1000}, caps: nil, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.InDelta(t, tt.want, pctOfNetworkCap(tt.fn, tt.caps), 1e-9)
		})
	}
}
