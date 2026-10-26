package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

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

// SimulateTransaction hits a Soroban RPC node to estimate resources for an XDR transaction envelope
func SimulateTransaction(rpcURL string, txXDR string) (*SimulateTransactionResponse, error) {
	reqBody := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "simulateTransaction",
		Params:  map[string]string{"transaction": txXDR},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(rpcURL, "application/json", bytes.NewBuffer(payload))
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

	return Resources{
		CPUInstructions: int64(txData.Resources.Instructions),
		ReadBytes:       int64(txData.Resources.DiskReadBytes),
		WriteBytes:      int64(txData.Resources.WriteBytes),
	}, nil
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
