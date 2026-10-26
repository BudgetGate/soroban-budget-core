package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/stellar/go-stellar-sdk/xdr"
)

// JSONRPCRequest represents a standard JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

// SimulateTransactionResponse represents the Soroban RPC response structure
type SimulateTransactionResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		// Cost is only returned by older RPC versions; current stellar-rpc omits it.
		Cost *struct {
			CPUInstructions string `json:"cpuInsns"`
			MemoryBytes     string `json:"memBytes"`
		} `json:"cost,omitempty"`
		Results []struct {
			Auth []interface{} `json:"auth"`
			XDR  string        `json:"xdr"`
		} `json:"results"`
		TransactionData string `json:"transactionData"`
		MinResourceFee  string `json:"minResourceFee"`
		// Error is set when the simulation itself failed (e.g. a contract panic).
		Error string `json:"error"`
	} `json:"result,omitempty"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// rpcTimeout bounds a single simulateTransaction round trip, including reading the response.
const rpcTimeout = 30 * time.Second

// SimulateTransaction hits a Soroban RPC node to estimate resources for an XDR transaction envelope
func SimulateTransaction(ctx context.Context, rpcURL string, txXDR string) (*SimulateTransactionResponse, error) {
	return simulateTransaction(ctx, rpcURL, txXDR, 0)
}

// simulateTransaction calls simulateTransaction, passing resourceConfig.instructionLeeway when leeway > 0.
func simulateTransaction(ctx context.Context, rpcURL string, txXDR string, leeway int64) (*SimulateTransactionResponse, error) {
	params := map[string]interface{}{"transaction": txXDR}
	if leeway > 0 {
		params["resourceConfig"] = map[string]int64{"instructionLeeway": leeway}
	}

	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "simulateTransaction",
		Params:  params,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rpcURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: rpcTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rpc post failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var simResp SimulateTransactionResponse
	if err := json.Unmarshal(bodyBytes, &simResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if simResp.Error != nil {
		return nil, fmt.Errorf("rpc returned error: %d %s", simResp.Error.Code, simResp.Error.Message)
	}

	if simResp.Result.Error != "" {
		return nil, fmt.Errorf("simulation failed: %s", simResp.Result.Error)
	}

	return &simResp, nil
}

// Resources holds the resource figures declared in a simulation's SorobanTransactionData.
type Resources struct {
	CPUInstructions int64
	ReadBytes       int64
	WriteBytes      int64
	// Footprint sizes. Since protocol 23, reads of live contract storage are not charged as
	// DiskReadBytes, so entry counts are the only signal for added contract-storage access.
	ReadEntries  int64 // every footprint entry (read-only and read-write)
	WriteEntries int64 // read-write footprint entries
}

// ParseTransactionData decodes the base64 SorobanTransactionData XDR returned by simulateTransaction.
func ParseTransactionData(b64 string) (Resources, error) {
	if b64 == "" {
		return Resources{}, fmt.Errorf("simulation returned no transactionData")
	}

	var txData xdr.SorobanTransactionData
	if err := xdr.SafeUnmarshalBase64(b64, &txData); err != nil {
		return Resources{}, fmt.Errorf("failed to decode transactionData XDR: %w", err)
	}

	footprint := txData.Resources.Footprint
	return Resources{
		CPUInstructions: int64(txData.Resources.Instructions),
		ReadBytes:       int64(txData.Resources.DiskReadBytes),
		WriteBytes:      int64(txData.Resources.WriteBytes),
		ReadEntries:     int64(len(footprint.ReadOnly) + len(footprint.ReadWrite)),
		WriteEntries:    int64(len(footprint.ReadWrite)),
	}, nil
}

// The RPC declares instructions as actual usage plus max(instructionLeeway, a provider-configured
// minimum pad; 50,000 on soroban-testnet). Simulating with two leeways above that pad makes the
// declared value exactly actual+leeway, so the pad can be removed without knowing its size.
const (
	measureLeewayLow  int64 = 1_000_000
	measureLeewayHigh int64 = 2_000_000
)

// MeasureResources simulates txXDR twice with known instruction leeways and returns the actual
// CPU instructions used, with read/write bytes as declared by the RPC. The first simulation's
// response is also returned for callers that need other fields from it.
func MeasureResources(ctx context.Context, rpcURL string, txXDR string) (Resources, *SimulateTransactionResponse, error) {
	lowResp, err := simulateTransaction(ctx, rpcURL, txXDR, measureLeewayLow)
	if err != nil {
		return Resources{}, nil, err
	}
	low, err := ParseTransactionData(lowResp.Result.TransactionData)
	if err != nil {
		return Resources{}, nil, err
	}

	highResp, err := simulateTransaction(ctx, rpcURL, txXDR, measureLeewayHigh)
	if err != nil {
		return Resources{}, nil, err
	}
	high, err := ParseTransactionData(highResp.Result.TransactionData)
	if err != nil {
		return Resources{}, nil, err
	}

	// If the leeway did not add exactly, the RPC pads differently than assumed (e.g. its
	// minimum pad exceeds measureLeewayLow), and subtracting the leeway would be wrong.
	if high.CPUInstructions-low.CPUInstructions != measureLeewayHigh-measureLeewayLow {
		return Resources{}, nil, fmt.Errorf(
			"cannot measure exact CPU instructions: declared %d with leeway %d and %d with leeway %d",
			low.CPUInstructions, measureLeewayLow, high.CPUInstructions, measureLeewayHigh)
	}

	low.CPUInstructions -= measureLeewayLow
	return low, lowResp, nil
}

// parseOptionalInt parses a numeric string, returning nil when the value is absent.
func parseOptionalInt(s string) (*int64, error) {
	if s == "" {
		return nil, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return &v, nil
}
