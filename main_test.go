package main

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		{name: "threshold above the rise", args: []string{"--baseline", base, "--new", regressed, "--max-delta-pct", "150"}, want: exitOK},
		{name: "no arguments", args: nil, want: exitError, wantStdout: "Usage:"},
		{name: "undefined flag", args: []string{"--bogus"}, want: exitError, wantStderr: "flag provided but not defined"},
		{name: "missing baseline file", args: []string{"--baseline", missing, "--new", same}, want: exitError, wantStdout: "Error opening baseline"},
		{name: "invalid baseline JSON", args: []string{"--baseline", badJSON, "--new", same}, want: exitError, wantStdout: "Error parsing baseline"},
		{name: "unsupported baseline version", args: []string{"--baseline", badVersion, "--new", same}, want: exitError, wantStderr: "unsupported snapshot schema_version"},
		{name: "neither --new nor --fixture", args: []string{"--baseline", base}, want: exitError, wantStdout: "Must provide either --new or --fixture."},
		{name: "missing new snapshot", args: []string{"--baseline", base, "--new", missing}, want: exitError, wantStdout: "Error opening new snapshot"},
		{name: "unsupported new version", args: []string{"--baseline", base, "--new", badVersion}, want: exitError, wantStderr: "Error reading new snapshot"},
		{name: "missing fixture", args: []string{"--baseline", base, "--fixture", missing, "--rpc-url", rpcOK.URL}, want: exitError, wantStdout: "Error opening fixture"},
		{name: "invalid fixture JSON", args: []string{"--baseline", base, "--fixture", badJSON, "--rpc-url", rpcOK.URL}, want: exitError, wantStdout: "Error decoding fixture JSON"},
		{name: "RPC failure", args: []string{"--baseline", base, "--fixture", fixture, "--rpc-url", rpcDown.URL}, want: exitError, wantStdout: "Failed to measure resources for f"},
		{name: "fixture regresses", args: []string{"--baseline", base, "--fixture", fixture, "--rpc-url", rpcOK.URL}, want: exitRegression, wantStdout: "| f | CPUInstructions | 1000 | 91406 |"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := run(tt.args, &stdout, &stderr)
			assert.Equal(t, tt.want, got, "stdout: %s\nstderr: %s", stdout.String(), stderr.String())
			assert.Contains(t, stdout.String(), tt.wantStdout)
			assert.Contains(t, stderr.String(), tt.wantStderr)
		})
	}
}
