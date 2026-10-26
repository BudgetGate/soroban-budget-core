package main

import (
	"context"
	"encoding/json"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Real transactionData returned by soroban-testnet for the balance_check fixture.
const testnetTxData = "AAAAAAAAAAIAAAAAAAAAAERy0MTnQCHX0yyz/340K5yMwzwaNKle8hPNNaniXdqcAAAABgAAAAHXkotywnA8z+r365/0701QSlWouXn8m0UOoshCtNHOYQAAABQAAAABAAAAAAACKF4AAACQAAAAAAAAAAAAABv+"

func TestSimulateTransaction_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"transactionData":"` + testnetTxData + `","cost":{"cpuInsns":"150000","memBytes":"2048"}}}`))
	}))
	defer ts.Close()

	resp, err := SimulateTransaction(context.Background(), ts.URL, "AAAAAAA...")
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

	resp, err := SimulateTransaction(context.Background(), ts.URL, "AAAAAAA...")
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

	resp, err := SimulateTransaction(context.Background(), ts.URL, "AAAAAAA...")
	assert.ErrorContains(t, err, "HostError")
	assert.Nil(t, resp)
}

func TestSimulateTransaction_Error(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	resp, err := SimulateTransaction(context.Background(), ts.URL, "AAAAAAA...")
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestSimulateTransaction_ContextDeadline(t *testing.T) {
	// A hung RPC must not block the CLI: the request fails when the context expires.
	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer ts.Close()
	defer close(release) // runs before ts.Close, which waits for the handler to return

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	resp, err := SimulateTransaction(ctx, ts.URL, "AAAAAAA...")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Nil(t, resp)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestParseTransactionData(t *testing.T) {
	res, err := ParseTransactionData(testnetTxData)
	assert.NoError(t, err)
	assert.Equal(t, int64(141406), res.CPUInstructions)
	assert.Equal(t, int64(144), res.ReadBytes)
	assert.Equal(t, int64(0), res.WriteBytes)
	assert.Equal(t, int64(2), res.ReadEntries) // native contract instance + source account
	assert.Equal(t, int64(0), res.WriteEntries)
}

// Real transactionData returned by soroban-testnet for the transfer_to_contract fixture.
const testnetTransferTxData = "AAAAAAAAAAEAAAAGAAAAAdeSi3LCcDzP6vfrn/TvTVBKVai5efybRQ6iyEK00c5hAAAAFAAAAAEAAAACAAAAAAAAAAAoKON9PeseeiJNc+mQc7AUgGCXCLXXuTxjyPIEi6aiMQAAAAYAAAAB15KLcsJwPM/q9+uf9O9NUEpVqLl5/JtFDqLIQrTRzmEAAAAQAAAAAQAAAAIAAAAPAAAAB0JhbGFuY2UAAAAAEgAAAAHXkotywnA8z+r365/0701QSlWouXn8m0UOoshCtNHOYQAAAAEABBl3AAAAkAAAAXAAAAAAAABpLA=="

func TestParseTransactionData_StorageWrite(t *testing.T) {
	res, err := ParseTransactionData(testnetTransferTxData)
	assert.NoError(t, err)
	assert.Equal(t, int64(268663), res.CPUInstructions) // includes the RPC's 50,000 pad
	assert.Equal(t, int64(368), res.WriteBytes)
	assert.Equal(t, int64(3), res.ReadEntries)
	assert.Equal(t, int64(2), res.WriteEntries) // source account + contract balance entry

	// The transfer also reads a 216-byte contract-data balance entry, yet DiskReadBytes is the
	// same as balance_check's: live contract storage is not charged as disk reads.
	assert.Equal(t, int64(144), res.ReadBytes)
}

func TestParseTransactionData_Invalid(t *testing.T) {
	_, err := ParseTransactionData("")
	assert.Error(t, err)

	_, err = ParseTransactionData("not-xdr")
	assert.Error(t, err)
}

// paddingRPC mimics soroban-testnet: declared instructions are actual + max(leeway, minPad).
func paddingRPC(t *testing.T, actual, minPad int64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				ResourceConfig struct {
					InstructionLeeway int64 `json:"instructionLeeway"`
				} `json:"resourceConfig"`
			} `json:"params"`
		}
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&req))

		pad := max(req.Params.ResourceConfig.InstructionLeeway, minPad)
		txData := xdr.SorobanTransactionData{
			Resources: xdr.SorobanResources{
				Instructions:  xdr.Uint32(actual + pad),
				DiskReadBytes: 144,
				WriteBytes:    52,
			},
		}
		b64, err := xdr.MarshalBase64(txData)
		assert.NoError(t, err)
		w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"transactionData":"` + b64 + `"}}`))
	}))
}

func TestMeasureResources_RemovesPadding(t *testing.T) {
	// Values observed on soroban-testnet for the balance_check fixture.
	ts := paddingRPC(t, 91406, 50000)
	defer ts.Close()

	res, resp, err := MeasureResources(context.Background(), ts.URL, "AAAAAAA...")
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(91406), res.CPUInstructions)
	assert.Equal(t, int64(144), res.ReadBytes)
	assert.Equal(t, int64(52), res.WriteBytes)
}

func TestMeasureResources_PadLargerThanLeeway(t *testing.T) {
	// A provider whose minimum pad exceeds our leeway must not yield a silently wrong figure.
	ts := paddingRPC(t, 91406, 1_500_000)
	defer ts.Close()

	_, _, err := MeasureResources(context.Background(), ts.URL, "AAAAAAA...")
	assert.ErrorContains(t, err, "cannot measure exact CPU instructions")
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
