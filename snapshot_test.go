package main

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestParseSnapshot(t *testing.T) {
	jsonStr := `
	{
		"schema_version": "1.0",
		"network": "testnet",
		"generated_at": "2026-08-06T12:00:00Z",
		"commit_sha": "abc1234",
		"functions": [
			{
				"name": "mint",
				"fixture_id": "test1",
				"cpu_instructions": 1000,
				"memory_bytes": 500,
				"read_bytes": 200,
				"write_bytes": 100,
				"pct_of_network_cap": 0.05
			}
		]
	}`

	r := strings.NewReader(jsonStr)
	snapshot, err := ParseSnapshot(r)

	assert.NoError(t, err)
	assert.NotNil(t, snapshot)
	assert.Equal(t, "1.0", snapshot.SchemaVersion)
	assert.Equal(t, "testnet", snapshot.Network)
	assert.Equal(t, "abc1234", snapshot.CommitSHA)

	assert.Len(t, snapshot.Functions, 1)
	fn := snapshot.Functions[0]
	assert.Equal(t, "mint", fn.Name)
	assert.Equal(t, int64(1000), fn.CPUInstructions)
	assert.Equal(t, int64(500), *fn.MemoryBytes)
	assert.Equal(t, 0.05, fn.PctOfNetworkCap)
}

func TestParseSnapshot_NullMemory(t *testing.T) {
	jsonStr := `{"schema_version": "1.0", "functions": [{"name": "mint", "memory_bytes": null}]}`
	snapshot, err := ParseSnapshot(strings.NewReader(jsonStr))

	assert.NoError(t, err)
	assert.Nil(t, snapshot.Functions[0].MemoryBytes)
}

func TestParseSnapshot_SchemaVersion(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
	}{
		{name: "1.0 is read", json: `{"schema_version": "1.0", "functions": []}`},
		{name: "current version is read", json: `{"schema_version": "` + CurrentSchemaVersion + `", "functions": []}`},
		{name: "missing version", json: `{"functions": []}`, wantErr: true},
		{name: "empty version", json: `{"schema_version": "", "functions": []}`, wantErr: true},
		{name: "unknown version", json: `{"schema_version": "3.0", "functions": []}`, wantErr: true},
		{name: "unnormalised version", json: `{"schema_version": "2", "functions": []}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot, err := ParseSnapshot(strings.NewReader(tt.json))
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrUnsupportedSchema)
				assert.Nil(t, snapshot)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, snapshot)
		})
	}
}

func TestParseSnapshot_InvalidJSON(t *testing.T) {
	jsonStr := `{ "schema_version": "1.0", "network": }`
	r := strings.NewReader(jsonStr)
	snapshot, err := ParseSnapshot(r)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
}
