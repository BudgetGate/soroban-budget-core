package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
)

// CurrentSchemaVersion is the snapshot schema version core writes. See SCHEMA.md.
const CurrentSchemaVersion = "2.0"

// ErrUnsupportedSchema is returned by ParseSnapshot for a missing or unknown schema_version.
var ErrUnsupportedSchema = errors.New("unsupported snapshot schema_version")

type FixtureEntry struct {
	Name      string `json:"name"`
	FixtureID string `json:"fixture_id"`
	XDR       string `json:"xdr"`
}

type FunctionCost struct {
	Name            string  `json:"name"`
	FixtureID       string  `json:"fixture_id"`
	CPUInstructions int64   `json:"cpu_instructions"`
	MemoryBytes     *int64  `json:"memory_bytes"` // nil when the RPC does not report memory
	ReadBytes       int64   `json:"read_bytes"`
	WriteBytes      int64   `json:"write_bytes"`
	ReadEntries     *int64  `json:"read_entries"`  // nil in snapshots written before entry counts existed
	WriteEntries    *int64  `json:"write_entries"` // nil in snapshots written before entry counts existed
	PctOfNetworkCap float64 `json:"pct_of_network_cap"`
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
	if err := checkSchemaVersion(s.SchemaVersion); err != nil {
		return nil, err
	}
	return &s, nil
}

// duplicateNames returns each name that occurs more than once in items, once, in the order the
// repeats appear. Snapshots are compared by function name, so a repeated name is ambiguous.
func duplicateNames[T any](items []T, name func(T) string) []string {
	seen := make(map[string]int)
	var dups []string
	for _, item := range items {
		n := name(item)
		seen[n]++
		if seen[n] == 2 {
			dups = append(dups, n)
		}
	}
	return dups
}

func functionName(fn FunctionCost) string { return fn.Name }

// withoutNames returns funcs minus every function whose name is in names.
func withoutNames(funcs []FunctionCost, names []string) []FunctionCost {
	var kept []FunctionCost
	for _, fn := range funcs {
		if !slices.Contains(names, fn.Name) {
			kept = append(kept, fn)
		}
	}
	return kept
}

// quotedList formats names as "a", "b" for messages.
func quotedList(names []string) string {
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = strconv.Quote(n)
	}
	return strings.Join(quoted, ", ")
}

// checkSchemaVersion accepts the versions core can compare. 1.0 snapshots differ from 2.0 only
// in ways the comparison already handles: memory_bytes may be a number (it is not compared) and
// read_entries/write_entries may be absent (reported as unavailable).
func checkSchemaVersion(v string) error {
	switch v {
	case "1.0", CurrentSchemaVersion:
		return nil
	case "":
		return fmt.Errorf("%w: missing", ErrUnsupportedSchema)
	}
	return fmt.Errorf("%w: %q (this build reads 1.0 and %s)", ErrUnsupportedSchema, v, CurrentSchemaVersion)
}
