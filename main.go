package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

// Exit codes. A regression must be distinguishable from a run that could not compare at all.
const (
	exitOK         = 0
	exitRegression = 1
	exitError      = 2 // usage or runtime error
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the CLI and returns its exit code. stdout carries only the Markdown report, so
// callers can use it as-is; progress and error messages go to stderr.
func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("budget-core", flag.ContinueOnError)
	fs.SetOutput(stderr)
	baselinePath := fs.String("baseline", "", "Path to baseline snapshot JSON")
	newPath := fs.String("new", "", "Path to new snapshot JSON (optional if using fixture)")
	fixturePath := fs.String("fixture", "", "Path to fixture JSON containing XDRs for simulation")
	rpcURL := fs.String("rpc-url", "https://soroban-testnet.stellar.org", "Soroban RPC URL")
	outputJson := fs.String("output-json", "", "Path to write the new snapshot JSON to")
	maxDeltaPct := fs.Float64("max-delta-pct", 10.0, "Maximum allowed percentage increase in resource usage")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}

	if *baselinePath == "" {
		fmt.Fprintln(stderr, "Usage: soroban-budget-core --baseline <file> [--new <file> | --fixture <file>]")
		return exitError
	}

	baseline, code := readSnapshot("baseline", *baselinePath, stderr)
	if code != exitOK {
		return code
	}

	var newSnapshot *Snapshot

	if *fixturePath != "" {
		fmt.Fprintf(stderr, "Simulating transactions from fixture via %s...\n", *rpcURL)

		fixtureFile, err := os.Open(*fixturePath)
		if err != nil {
			fmt.Fprintf(stderr, "Error opening fixture: %v\n", err)
			return exitError
		}
		defer fixtureFile.Close()

		var fixtures []FixtureEntry
		if err := json.NewDecoder(fixtureFile).Decode(&fixtures); err != nil {
			fmt.Fprintf(stderr, "Error decoding fixture JSON: %v\n", err)
			return exitError
		}

		var funcs []FunctionCost
		for _, f := range fixtures {
			res, simResp, err := MeasureResources(context.Background(), *rpcURL, f.XDR)
			if err != nil {
				fmt.Fprintf(stderr, "Failed to measure resources for %s: %v\n", f.Name, err)
				return exitError
			}

			// Memory is only available from the legacy cost field; record it as unknown otherwise.
			var memBytes *int64
			if simResp.Result.Cost != nil {
				memBytes, err = parseOptionalInt(simResp.Result.Cost.MemoryBytes)
				if err != nil {
					fmt.Fprintf(stderr, "Invalid memBytes for %s: %v\n", f.Name, err)
					return exitError
				}
			}

			funcs = append(funcs, FunctionCost{
				Name:            f.Name,
				FixtureID:       f.FixtureID,
				CPUInstructions: res.CPUInstructions,
				MemoryBytes:     memBytes,
				ReadBytes:       res.ReadBytes,
				WriteBytes:      res.WriteBytes,
				ReadEntries:     &res.ReadEntries,
				WriteEntries:    &res.WriteEntries,
			})
		}

		newSnapshot = &Snapshot{
			SchemaVersion: CurrentSchemaVersion,
			Network:       "testnet",
			CommitSHA:     "dynamic",
			Functions:     funcs,
		}
	} else if *newPath != "" {
		newSnapshot, code = readSnapshot("new snapshot", *newPath, stderr)
		if code != exitOK {
			return code
		}
	} else {
		fmt.Fprintln(stderr, "Must provide either --new or --fixture.")
		return exitError
	}

	thresholds := Thresholds{MaxDeltaPct: *maxDeltaPct}
	results := CompareSnapshots(baseline, newSnapshot, thresholds)

	markdown := FormatDiffs(results)
	fmt.Fprint(stdout, markdown)

	if *outputJson != "" {
		data, err := json.MarshalIndent(newSnapshot, "", "  ")
		if err == nil {
			_ = os.WriteFile(*outputJson, data, 0644)
		}
	}

	for _, d := range results {
		if d.Regression {
			return exitRegression
		}
	}
	return exitOK
}

// readSnapshot opens and parses a snapshot file. On failure it prints the error to stderr and
// returns exitError.
func readSnapshot(label, path string, stderr io.Writer) (*Snapshot, int) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(stderr, "Error opening %s: %v\n", label, err)
		return nil, exitError
	}
	defer f.Close()

	s, err := ParseSnapshot(f)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading %s %s: %v\n", label, path, err)
		return nil, exitError
	}
	return s, exitOK
}
