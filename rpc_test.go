package main

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Real transactionData returned by soroban-testnet for the balance_check fixture.
const testnetTxData = "AAAAAAAAAAIAAAAAAAAAAERy0MTnQCHX0yyz/340K5yMwzwaNKle8hPNNaniXdqcAAAABgAAAAHXkotywnA8z+r365/0701QSlWouXn8m0UOoshCtNHOYQAAABQAAAABAAAAAAACKF4AAACQAAAAAAAAAAAAABv+"

func TestSimulateTransaction_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"transactionData":"` + testnetTxData + `","cost":{"cpuInsns":"150000","memBytes":"2048"}}}`))
	}))
	defer ts.Close()

	resp, err := SimulateTransaction(ts.URL, "AAAAAAA...")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, testnetTxData, resp.Result.TransactionData)
	assert.NotNil(t, resp.Result.Cost)
	assert.Equal(t, "2048", resp.Result.Cost.MemoryBytes)
}

func TestSimulateTransaction_NoCostField(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"transactionData":"` + testnetTxData + `"}}`))
	}))
	defer ts.Close()

	resp, err := SimulateTransaction(ts.URL, "AAAAAAA...")
	assert.NoError(t, err)
	assert.Nil(t, resp.Result.Cost)
}

func TestSimulateTransaction_SimulationError(t *testing.T) {
	// A failed simulation is reported in result.error, not as a JSON-RPC error.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]interface{}{"error": "HostError: Error(WasmVm, InvalidAction)"},
		})
	}))
	defer ts.Close()

	resp, err := SimulateTransaction(ts.URL, "AAAAAAA...")
	assert.ErrorContains(t, err, "HostError")
	assert.Nil(t, resp)
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

func TestParseTransactionData(t *testing.T) {
	res, err := ParseTransactionData(testnetTxData)
	assert.NoError(t, err)
	assert.Equal(t, int64(141406), res.CPUInstructions)
	assert.Equal(t, int64(144), res.ReadBytes)
	assert.Equal(t, int64(0), res.WriteBytes)
}

func TestParseTransactionData_Invalid(t *testing.T) {
	_, err := ParseTransactionData("")
	assert.Error(t, err)

	_, err = ParseTransactionData("not-xdr")
	assert.Error(t, err)
}

func TestParseOptionalInt(t *testing.T) {
	v, err := parseOptionalInt("")
	assert.NoError(t, err)
	assert.Nil(t, v)

	v, err = parseOptionalInt("2048")
	assert.NoError(t, err)
	assert.Equal(t, int64(2048), *v)

	_, err = parseOptionalInt("abc")
	assert.Error(t, err)
}
