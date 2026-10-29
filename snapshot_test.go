package main

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSnapshot(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		wantErr   bool
		wantErrIs error
		check     func(t *testing.T, s *Snapshot)
	}{
		{
			name: "1.0 snapshot with every field",
			json: `{
				"schema_version": "1.0",
				"network": "testnet",
				"generated_at": "2026-08-06T12:00:00Z",
				"commit_sha": "abc1234",
				"functions": [{"name": "mint", "fixture_id": "test1", "cpu_instructions": 1000, "memory_bytes": 500,
					"read_bytes": 200, "write_bytes": 100, "pct_of_network_cap": 0.05}]
			}`,
			check: func(t *testing.T, s *Snapshot) {
				assert.Equal(t, "1.0", s.SchemaVersion)
				assert.Equal(t, "testnet", s.Network)
				assert.Equal(t, "abc1234", s.CommitSHA)
				require.Len(t, s.Functions, 1)
				fn := s.Functions[0]
				assert.Equal(t, "mint", fn.Name)
				assert.Equal(t, int64(1000), fn.CPUInstructions)
				assert.Equal(t, int64Ptr(500), fn.MemoryBytes)
				assert.Equal(t, 0.05, fn.PctOfNetworkCap)
				assert.Nil(t, fn.ReadEntries) // 1.0 snapshots may lack entry counts
			},
		},
		{
			name: "null memory and entry counts",
			json: `{"schema_version": "2.0", "functions": [{"name": "mint", "memory_bytes": null, "read_entries": 3, "write_entries": 2}]}`,
			check: func(t *testing.T, s *Snapshot) {
				fn := s.Functions[0]
				assert.Nil(t, fn.MemoryBytes)
				assert.Equal(t, int64Ptr(3), fn.ReadEntries)
				assert.Equal(t, int64Ptr(2), fn.WriteEntries)
			},
		},
		{name: "1.0 is read", json: `{"schema_version": "1.0", "functions": []}`},
		{name: "current version is read", json: `{"schema_version": "` + CurrentSchemaVersion + `", "functions": []}`},
		{name: "missing version", json: `{"functions": []}`, wantErrIs: ErrUnsupportedSchema},
		{name: "empty version", json: `{"schema_version": "", "functions": []}`, wantErrIs: ErrUnsupportedSchema},
		{name: "unknown version", json: `{"schema_version": "3.0", "functions": []}`, wantErrIs: ErrUnsupportedSchema},
		{name: "unnormalised version", json: `{"schema_version": "2", "functions": []}`, wantErrIs: ErrUnsupportedSchema},
		{name: "invalid JSON", json: `{ "schema_version": "1.0", "network": }`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot, err := ParseSnapshot(strings.NewReader(tt.json))
			if tt.wantErr || tt.wantErrIs != nil {
				assert.Error(t, err)
				if tt.wantErrIs != nil {
					assert.ErrorIs(t, err, tt.wantErrIs)
				}
				assert.Nil(t, snapshot)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, snapshot)
			if tt.check != nil {
				tt.check(t, snapshot)
			}
		})
	}
}

// TestTestdataSamples keeps the sample files in testdata/ readable by the current code.
func TestTestdataSamples(t *testing.T) {
	t.Run("fixture", func(t *testing.T) {
		b, err := os.ReadFile(filepath.Join("testdata", "fixture.json"))
		require.NoError(t, err)
		var fixtures []FixtureEntry
		require.NoError(t, json.Unmarshal(b, &fixtures))
		require.NotEmpty(t, fixtures)
		for _, f := range fixtures {
			assert.NotEmpty(t, f.Name)
			assert.NotEmpty(t, f.FixtureID)
			assert.NotEmpty(t, f.XDR)
		}
	})

	t.Run("counter fixtures", func(t *testing.T) {
		for _, name := range []string{"fixture_v1.json", "fixture_v2.json"} {
			b, err := os.ReadFile(filepath.Join("testdata", "counter", name))
			require.NoError(t, err)
			var fixtures []FixtureEntry
			require.NoError(t, json.Unmarshal(b, &fixtures), name)
			require.Len(t, fixtures, 1, name)
			assert.Equal(t, "bump", fixtures[0].Name, name)
			assert.NotEmpty(t, fixtures[0].XDR, name)
		}
	})

	t.Run("snapshot", func(t *testing.T) {
		f, err := os.Open(filepath.Join("testdata", "snapshot_testnet.json"))
		require.NoError(t, err)
		defer f.Close()
		s, err := ParseSnapshot(f)
		require.NoError(t, err)
		assert.Equal(t, CurrentSchemaVersion, s.SchemaVersion)
		assert.NotEmpty(t, s.Functions)
	})
}

func TestDuplicateNames(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  []string
	}{
		{name: "none", names: []string{"a", "b"}, want: nil},
		{name: "empty", names: nil, want: nil},
		{name: "one repeated", names: []string{"a", "b", "a"}, want: []string{"a"}},
		{name: "listed once however often repeated", names: []string{"a", "a", "a"}, want: []string{"a"}},
		{name: "in the order the repeats appear", names: []string{"b", "a", "a", "b"}, want: []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcs := make([]FunctionCost, len(tt.names))
			for i, n := range tt.names {
				funcs[i] = FunctionCost{Name: n}
			}
			assert.Equal(t, tt.want, duplicateNames(funcs, functionName))
		})
	}
}

func TestWithoutNames(t *testing.T) {
	funcs := []FunctionCost{{Name: "a"}, {Name: "b"}, {Name: "a"}, {Name: "c"}}
	assert.Equal(t, []FunctionCost{{Name: "b"}, {Name: "c"}}, withoutNames(funcs, []string{"a"}))
	assert.Equal(t, `"a", "b"`, quotedList([]string{"a", "b"}))
}
