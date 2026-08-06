package main

import (
	"encoding/json"
	"io"
)

type FunctionCost struct {
	Name             string `json:"name"`
	FixtureID        string `json:"fixture_id"`
	CPUInstructions  int64  `json:"cpu_instructions"`
	MemoryBytes      int64  `json:"memory_bytes"`
	ReadBytes        int64  `json:"read_bytes"`
	WriteBytes       int64  `json:"write_bytes"`
	PctOfNetworkCap  float64 `json:"pct_of_network_cap"`
}

type Snapshot struct {
	SchemaVersion string         `json:"schema_version"`
	Network       string         `json:"network"`
	GeneratedAt   string         `json:"generated_at"`
	CommitSHA     string         `json:"commit_sha"`
	Functions     []FunctionCost `json:"functions"`
}

func ParseSnapshot(r io.Reader) (*Snapshot, error) {
	var s Snapshot
	err := json.NewDecoder(r).Decode(&s)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
