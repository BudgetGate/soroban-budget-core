package main

import (
	"bytes"
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRunExitCodes(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
		return p
	}

	snapshot := `{"schema_version": "2.0", "functions": [{"name": "f", "cpu_instructions": 1000, "read_bytes": 144, "write_bytes": 0, "read_entries": 2, "write_entries": 0}]}`
	base := write("base.json", snapshot)
	same := write("same.json", snapshot)
	withExtra := write("extra.json", `{"schema_version": "2.0", "functions": [{"name": "f", "cpu_instructions": 1000, "read_bytes": 144, "write_bytes": 0, "read_entries": 2, "write_entries": 0}, {"name": "g", "cpu_instructions": 5000, "read_bytes": 144, "write_bytes": 0, "read_entries": 2, "write_entries": 0}]}`)
	regressed := write("regressed.json", `{"schema_version": "2.0", "functions": [{"name": "f", "cpu_instructions": 2000, "read_bytes": 144, "write_bytes": 0, "read_entries": 2, "write_entries": 0}]}`)
	badJSON := write("bad.json", `{bad`)
	badVersion := write("v3.json", `{"schema_version": "3.0", "functions": []}`)
	fixture := write("fixture.json", `[{"name": "f", "fixture_id": "t", "xdr": "AAAA"}]`)
	missing := filepath.Join(dir, "missing.json")

	// The fake RPC reports 91,406 actual instructions, so the fixture run regresses against base.
	rpcOK := paddingRPC(t, 91406, 50000)
	defer rpcOK.Close()
	rpcDown := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer rpcDown.Close()
	// Serves network caps but fails every simulation.
	rpcSimFails := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		switch req.Method {
		case "getLedgerEntries":
			w.Write(capturedFile(t, "getledgerentries_config.json"))
		case "getNetwork":
			w.Write(capturedFile(t, "getnetwork.json"))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer rpcSimFails.Close()

	tests := []struct {
		name       string
		args       []string
		want       int
		wantStdout string
		wantStderr string
	}{
		{name: "help", args: []string{"-h"}, want: exitOK},
		{name: "no change", args: []string{"--baseline", base, "--new", same}, want: exitOK, wantStdout: "| f | CPUInstructions | 1000 | 1000 |"},
		{name: "regression", args: []string{"--baseline", base, "--new", regressed}, want: exitRegression, wantStdout: "❌ REGRESSION"},
		{name: "function added", args: []string{"--baseline", base, "--new", withExtra}, want: exitOK, wantStdout: "| g | ReadEntries | n/a | 2 | n/a | n/a | 0.50% | 🆕 NEW |"},
		{name: "function removed", args: []string{"--baseline", withExtra, "--new", same}, want: exitOK, wantStdout: "| g | — | n/a | n/a | n/a | n/a | n/a | 🗑️ REMOVED |"},
		{name: "threshold above the rise", args: []string{"--baseline", base, "--new", regressed, "--max-delta-pct", "150"}, want: exitOK},
		{name: "absolute CPU limit exceeded", args: []string{"--baseline", base, "--new", regressed, "--max-delta-pct", "0", "--max-cpu-increase", "500"}, want: exitRegression, wantStdout: "❌ REGRESSION"},
		{name: "absolute CPU limit not exceeded", args: []string{"--baseline", base, "--new", regressed, "--max-delta-pct", "0", "--max-cpu-increase", "5000"}, want: exitOK},
		{name: "cap warning level from flag (does not fail by default)", args: []string{"--baseline", base, "--new", same, "--cap-warning-pct", "0.0001"}, want: exitOK, wantStdout: "⚠️ CAP WARNING"},
		{name: "negative threshold", args: []string{"--baseline", base, "--new", same, "--max-read-entries-increase", "-1"}, want: exitError, wantStderr: "Thresholds must not be negative."},
		{name: "no arguments", args: nil, want: exitError, wantStderr: "Usage:"},
		{name: "undefined flag", args: []string{"--bogus"}, want: exitError, wantStderr: "flag provided but not defined"},
		{name: "missing baseline file", args: []string{"--baseline", missing, "--new", same}, want: exitError, wantStderr: "Error opening baseline"},
		{name: "invalid baseline JSON", args: []string{"--baseline", badJSON, "--new", same}, want: exitError, wantStderr: "Error reading baseline"},
		{name: "unsupported baseline version", args: []string{"--baseline", badVersion, "--new", same}, want: exitError, wantStderr: "unsupported snapshot schema_version"},
		{name: "neither --new nor --fixture", args: []string{"--baseline", base}, want: exitError, wantStderr: "Must provide either --new or --fixture."},
		{name: "missing new snapshot", args: []string{"--baseline", base, "--new", missing}, want: exitError, wantStderr: "Error opening new snapshot"},
		{name: "unsupported new version", args: []string{"--baseline", base, "--new", badVersion}, want: exitError, wantStderr: "Error reading new snapshot"},
		{name: "missing fixture", args: []string{"--baseline", base, "--fixture", missing, "--rpc-url", rpcOK.URL}, want: exitError, wantStderr: "Error opening fixture"},
		{name: "invalid fixture JSON", args: []string{"--baseline", base, "--fixture", badJSON, "--rpc-url", rpcOK.URL}, want: exitError, wantStderr: "Error decoding fixture JSON"},
		{name: "RPC failure", args: []string{"--baseline", base, "--fixture", fixture, "--rpc-url", rpcSimFails.URL}, want: exitError, wantStderr: "Failed to measure resources for f"},
		{name: "network caps unavailable, fixture", args: []string{"--baseline", base, "--fixture", fixture, "--rpc-url", rpcDown.URL}, want: exitError, wantStderr: "Error reading network caps"},
		{name: "network caps unavailable, --new", args: []string{"--baseline", base, "--new", same, "--rpc-url", rpcDown.URL}, want: exitError, wantStderr: "Error reading network caps"},
		{name: "offline --new with cap warnings off", args: []string{"--baseline", base, "--new", same, "--cap-warning-pct", "0", "--rpc-url", rpcDown.URL}, want: exitOK, wantStdout: "| f | CPUInstructions | 1000 | 1000 | 0 | 0.00% | n/a | ✅ OK |"},
		{name: "cap warning fails with --fail-on-cap-warning", args: []string{"--baseline", base, "--new", same, "--cap-warning-pct", "0.0001", "--fail-on-cap-warning"}, want: exitRegression, wantStdout: "⚠️ CAP WARNING", wantStderr: "--fail-on-cap-warning is set"},
		{name: "no cap warning with --fail-on-cap-warning", args: []string{"--baseline", base, "--new", same, "--fail-on-cap-warning"}, want: exitOK},
		{name: "--fail-on-cap-warning without warnings enabled", args: []string{"--baseline", base, "--new", same, "--cap-warning-pct", "0", "--fail-on-cap-warning"}, want: exitError, wantStderr: "--fail-on-cap-warning needs --cap-warning-pct above 0."},
		{name: "caps provenance under the table", args: []string{"--baseline", base, "--new", same}, want: exitOK, wantStdout: "% of Cap uses the network's per-transaction limits at protocol 29, ledger 5406471."},
		{name: "fixture regresses", args: []string{"--baseline", base, "--fixture", fixture, "--rpc-url", rpcOK.URL}, want: exitRegression, wantStdout: "| f | CPUInstructions | 1000 | 91406 |", wantStderr: "Simulating transactions from fixture"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Point every run at the fake RPC unless the case picks its own, so no test reaches the network.
			args := tt.args
			if !slices.Contains(args, "--rpc-url") {
				args = append(slices.Clone(args), "--rpc-url", rpcOK.URL)
			}
			var stdout, stderr bytes.Buffer
			got := run(args, &stdout, &stderr)
			assert.Equal(t, tt.want, got, "stdout: %s\nstderr: %s", stdout.String(), stderr.String())
			assert.Contains(t, stdout.String(), tt.wantStdout)
			assert.Contains(t, stderr.String(), tt.wantStderr)

			// stdout carries the report and nothing else, so callers can use it as-is.
			// (TestRunOutputJSON covers the one error that comes after the report.)
			switch {
			case tt.want == exitError || tt.name == "help":
				assert.Empty(t, stdout.String())
			default:
				assert.True(t, strings.HasPrefix(stdout.String(), "| Function | Metric |"), "stdout: %q", stdout.String())
			}
		})
	}
}

// TestCounterContractSnapshots compares the snapshots core measured on testnet for the counter
// contract (testdata/counter): v2 adds storage writes and hashing, so every cost but read bytes rises.
func TestCounterContractSnapshots(t *testing.T) {
	v1 := filepath.Join("testdata", "counter", "snapshot_v1.json")
	v2 := filepath.Join("testdata", "counter", "snapshot_v2.json")
	// Serves the captured protocol-29 caps; no simulation happens with --new.
	rpc := paddingRPC(t, 1000, 50000)
	defer rpc.Close()

	tests := []struct {
		name     string
		baseline string
		new      string
		want     int
		wantRows []string
	}{
		{
			name: "v1 to v2 regresses", baseline: v1, new: v2, want: exitRegression,
			wantRows: []string{
				"| bump | CPUInstructions | 291573 | 449005 | 157432 | 53.99% | 0.11% | ❌ REGRESSION |",
				"| bump | ReadBytes | 0 | 0 | 0 | 0.00% | 0.00% | ✅ OK |",
				"| bump | WriteBytes | 80 | 380 | 300 | 375.00% | 0.29% | ❌ REGRESSION |",
				"| bump | ReadEntries | 3 | 6 | 3 | 100.00% | 1.50% | ❌ REGRESSION |",
				"| bump | WriteEntries | 1 | 4 | 3 | 300.00% | 2.00% | ❌ REGRESSION |",
			},
		},
		{
			name: "v1 against itself passes", baseline: v1, new: v1, want: exitOK,
			wantRows: []string{"| bump | CPUInstructions | 291573 | 291573 | 0 | 0.00% | 0.07% | ✅ OK |"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := run([]string{"--baseline", tt.baseline, "--new", tt.new, "--rpc-url", rpc.URL}, &stdout, &stderr)
			assert.Equal(t, tt.want, got, "stdout: %s\nstderr: %s", stdout.String(), stderr.String())
			for _, row := range tt.wantRows {
				assert.Contains(t, stdout.String(), row)
			}
		})
	}
}

func TestRunOutputJSON(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base.json")
	require.NoError(t, os.WriteFile(base, []byte(`{"schema_version": "2.0", "functions": [{"name": "f", "cpu_instructions": 1000, "read_bytes": 144, "write_bytes": 52, "read_entries": 0, "write_entries": 0}]}`), 0o644))
	fixture := filepath.Join(dir, "fixture.json")
	require.NoError(t, os.WriteFile(fixture, []byte(`[{"name": "f", "fixture_id": "t", "xdr": "AAAA"}]`), 0o644))
	// The fake RPC reports the same costs as base: 1,000 instructions, 144 read bytes, 52 write bytes, no entries.
	rpc := paddingRPC(t, 1000, 50000)
	defer rpc.Close()

	tests := []struct {
		name       string
		out        string
		want       int
		wantStderr string
	}{
		{name: "written", out: filepath.Join(dir, "new.json"), want: exitOK},
		{name: "directory does not exist", out: filepath.Join(dir, "missing", "new.json"), want: exitError, wantStderr: "Error writing --output-json"},
		{name: "path is a directory", out: dir, want: exitError, wantStderr: "Error writing --output-json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			before := time.Now().UTC().Truncate(time.Second)
			got := run([]string{"--baseline", base, "--fixture", fixture, "--rpc-url", rpc.URL, "--output-json", tt.out, "--commit-sha", "abc1234"}, &stdout, &stderr)
			assert.Equal(t, tt.want, got, "stderr: %s", stderr.String())
			assert.Contains(t, stderr.String(), tt.wantStderr)
			// The report is printed before the snapshot is written, so it is there either way.
			assert.True(t, strings.HasPrefix(stdout.String(), "| Function | Metric |"), "stdout: %q", stdout.String())
			if tt.want != exitOK {
				return
			}
			f, err := os.Open(tt.out)
			require.NoError(t, err)
			defer f.Close()
			s, err := ParseSnapshot(f)
			require.NoError(t, err)
			assert.Equal(t, CurrentSchemaVersion, s.SchemaVersion)
			require.Len(t, s.Functions, 1)
			assert.Equal(t, int64(1000), s.Functions[0].CPUInstructions)
			// Highest share of a cap: 144 read bytes of the 200,000 testnet limit.
			assert.InDelta(t, 0.072, s.Functions[0].PctOfNetworkCap, 1e-9)

			// Metadata: network from the RPC's passphrase, the --commit-sha value, and the time of the run.
			assert.Equal(t, "testnet", s.Network)
			assert.Equal(t, "abc1234", s.CommitSHA)
			generated, err := time.Parse(time.RFC3339, s.GeneratedAt)
			require.NoError(t, err)
			assert.False(t, generated.Before(before), "generated_at %s is before the run started", s.GeneratedAt)
			assert.WithinDuration(t, time.Now(), generated, time.Minute)
		})
	}
}
