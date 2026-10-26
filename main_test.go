package main

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
		{name: "function added", args: []string{"--baseline", base, "--new", withExtra}, want: exitOK, wantStdout: "| g | CPUInstructions | n/a | 5000 | n/a | n/a | 0.01% | 🆕 NEW |"},
		{name: "function removed", args: []string{"--baseline", withExtra, "--new", same}, want: exitOK, wantStdout: "| g | — | n/a | n/a | n/a | n/a | n/a | 🗑️ REMOVED |"},
		{name: "threshold above the rise", args: []string{"--baseline", base, "--new", regressed, "--max-delta-pct", "150"}, want: exitOK},
		{name: "absolute CPU limit exceeded", args: []string{"--baseline", base, "--new", regressed, "--max-delta-pct", "0", "--max-cpu-increase", "500"}, want: exitRegression, wantStdout: "❌ REGRESSION"},
		{name: "absolute CPU limit not exceeded", args: []string{"--baseline", base, "--new", regressed, "--max-delta-pct", "0", "--max-cpu-increase", "5000"}, want: exitOK},
		{name: "cap warning level from flag", args: []string{"--baseline", base, "--new", same, "--cap-warning-pct", "0.0001"}, want: exitOK, wantStdout: "⚠️ CAP WARNING"},
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
		{name: "RPC failure", args: []string{"--baseline", base, "--fixture", fixture, "--rpc-url", rpcDown.URL}, want: exitError, wantStderr: "Failed to measure resources for f"},
		{name: "fixture regresses", args: []string{"--baseline", base, "--fixture", fixture, "--rpc-url", rpcOK.URL}, want: exitRegression, wantStdout: "| f | CPUInstructions | 1000 | 91406 |", wantStderr: "Simulating transactions from fixture"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := run(tt.args, &stdout, &stderr)
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
			got := run([]string{"--baseline", base, "--fixture", fixture, "--rpc-url", rpc.URL, "--output-json", tt.out}, &stdout, &stderr)
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
		})
	}
}
