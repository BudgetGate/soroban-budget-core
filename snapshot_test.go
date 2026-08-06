package main

import (
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
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
	assert.Equal(t, 0.05, fn.PctOfNetworkCap)
}

func TestParseSnapshot_InvalidJSON(t *testing.T) {
	jsonStr := `{ "schema_version": "1.0", "network": }`
	r := strings.NewReader(jsonStr)
	snapshot, err := ParseSnapshot(r)

	assert.Error(t, err)
	assert.Nil(t, snapshot)
}
