package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// JSONRPCRequest represents a standard JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

// SimulateTransactionResponse represents the Soroban RPC response structure
type SimulateTransactionResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Result  struct {
		Cost struct {
			CPUInstructions string `json:"cpuInsns"`
			MemoryBytes     string `json:"memBytes"`
		} `json:"cost"`
		Results []struct {
			Auth []interface{} `json:"auth"`
			XDR  string        `json:"xdr"`
		} `json:"results"`
		TransactionData string `json:"transactionData"`
		MinResourceFee  string `json:"minResourceFee"`
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
		Params:  []interface{}{txXDR},
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

	return &simResp, nil
}
