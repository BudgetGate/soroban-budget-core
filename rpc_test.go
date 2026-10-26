package main

import (
	"context"
	"encoding/json"
	"github.com/stellar/go-stellar-sdk/xdr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// capturedResponse returns a simulateTransaction response captured from soroban-testnet
// (see testdata/README.md).
func capturedResponse(t *testing.T, fixture string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "simulate_"+fixture+".json"))
	require.NoError(t, err)
	return b
}

// capturedTxData returns the transactionData of a captured simulateTransaction response.
func capturedTxData(t *testing.T, fixture string) string {
	t.Helper()
	var resp SimulateTransactionResponse
	require.NoError(t, json.Unmarshal(capturedResponse(t, fixture), &resp))
	require.NotEmpty(t, resp.Result.TransactionData)
	return resp.Result.TransactionData
}

func TestSimulateTransaction(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    func(t *testing.T) []byte
		wantErr string
		check   func(t *testing.T, resp *SimulateTransactionResponse)
	}{
		{
			name:   "captured balance_check response",
			status: http.StatusOK,
			body:   func(t *testing.T) []byte { return capturedResponse(t, "balance_check") },
			check: func(t *testing.T, resp *SimulateTransactionResponse) {
				assert.Equal(t, capturedTxData(t, "balance_check"), resp.Result.TransactionData)
				assert.Nil(t, resp.Result.Cost) // current RPCs omit the legacy cost field
			},
		},
		{
			name:   "captured transfer_to_contract response",
			status: http.StatusOK,
			body:   func(t *testing.T) []byte { return capturedResponse(t, "transfer_to_contract") },
			check: func(t *testing.T, resp *SimulateTransactionResponse) {
				assert.Equal(t, capturedTxData(t, "transfer_to_contract"), resp.Result.TransactionData)
			},
		},
		{
			// Older RPCs also returned a cost field. Current testnet does not, so this body is
			// hand-written around a captured transactionData.
			name:   "legacy cost field",
			status: http.StatusOK,
			body: func(t *testing.T) []byte {
				return []byte(`{"jsonrpc":"2.0","id":1,"result":{"transactionData":"` + capturedTxData(t, "balance_check") + `","cost":{"cpuInsns":"150000","memBytes":"2048"}}}`)
			},
			check: func(t *testing.T, resp *SimulateTransactionResponse) {
				require.NotNil(t, resp.Result.Cost)
				assert.Equal(t, "2048", resp.Result.Cost.MemoryBytes)
			},
		},
		{
			// A failed simulation is reported in result.error, not as a JSON-RPC error.
			name:   "simulation error",
			status: http.StatusOK,
			body: func(t *testing.T) []byte {
				return []byte(`{"jsonrpc":"2.0","id":1,"result":{"error":"HostError: Error(WasmVm, InvalidAction)"}}`)
			},
			wantErr: "HostError",
		},
		{
			name:   "JSON-RPC error",
			status: http.StatusOK,
			body: func(t *testing.T) []byte {
				return []byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"invalid parameters"}}`)
			},
			wantErr: "rpc returned error: -32602 invalid parameters",
		},
		{
			// The status code is not checked; an empty body fails at decoding.
			name:    "HTTP 500",
			status:  http.StatusInternalServerError,
			body:    func(t *testing.T) []byte { return nil },
			wantErr: "decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := tt.body(t)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				w.WriteHeader(tt.status)
				w.Write(body)
			}))
			defer ts.Close()

			resp, err := SimulateTransaction(context.Background(), ts.URL, "AAAAAAA...")
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				assert.Nil(t, resp)
				return
			}
			require.NoError(t, err)
			tt.check(t, resp)
		})
	}
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
	tests := []struct {
		name    string
		input   func(t *testing.T) string
		want    Resources
		wantErr bool
	}{
		{
			// Instructions include the RPC's 50,000 pad: this is the declared value.
			// Footprint: native contract instance + source account.
			name:  "captured balance_check",
			input: func(t *testing.T) string { return capturedTxData(t, "balance_check") },
			want:  Resources{CPUInstructions: 141406, ReadBytes: 144, WriteBytes: 0, ReadEntries: 2, WriteEntries: 0},
		},
		{
			// The transfer also reads a 216-byte contract-data balance entry, yet ReadBytes is the
			// same as balance_check's: live contract storage is not charged as disk reads.
			// Writes: source account + contract balance entry.
			name:  "captured transfer_to_contract",
			input: func(t *testing.T) string { return capturedTxData(t, "transfer_to_contract") },
			want:  Resources{CPUInstructions: 268663, ReadBytes: 144, WriteBytes: 368, ReadEntries: 3, WriteEntries: 2},
		},
		{name: "empty", input: func(t *testing.T) string { return "" }, wantErr: true},
		{name: "not XDR", input: func(t *testing.T) string { return "not-xdr" }, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := ParseTransactionData(tt.input(t))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, res)
		})
	}
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

func TestMeasureResources(t *testing.T) {
	tests := []struct {
		name    string
		minPad  int64
		wantErr string
	}{
		// Pad observed on soroban-testnet for the balance_check fixture.
		{name: "removes the provider pad", minPad: 50000},
		// A provider whose minimum pad exceeds our leeway must not yield a silently wrong figure.
		{name: "pad larger than leeway", minPad: 1_500_000, wantErr: "cannot measure exact CPU instructions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := paddingRPC(t, 91406, tt.minPad)
			defer ts.Close()

			res, resp, err := MeasureResources(context.Background(), ts.URL, "AAAAAAA...")
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, int64(91406), res.CPUInstructions)
			assert.Equal(t, int64(144), res.ReadBytes)
			assert.Equal(t, int64(52), res.WriteBytes)
		})
	}
}

func TestParseOptionalInt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *int64
		wantErr bool
	}{
		{name: "empty is unknown", input: "", want: nil},
		{name: "number", input: "2048", want: int64Ptr(2048)},
		{name: "not a number", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := parseOptionalInt(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, v)
		})
	}
}
