package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestSimulateTransaction_Success(t *testing.T) {
	mockResponse := SimulateTransactionResponse{
		JSONRPC: "2.0",
		ID:      1,
	}
	mockResponse.Result.Cost.CPUInstructions = "150000"
	mockResponse.Result.Cost.MemoryBytes = "2048"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer ts.Close()

	resp, err := SimulateTransaction(ts.URL, "AAAAAAA...")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "150000", resp.Result.Cost.CPUInstructions)
	assert.Equal(t, "2048", resp.Result.Cost.MemoryBytes)
}

func TestSimulateTransaction_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	resp, err := SimulateTransaction(ts.URL, "AAAAAAA...")
	assert.Error(t, err)
	assert.Nil(t, resp)
}
