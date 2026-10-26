package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	baselinePath := flag.String("baseline", "", "Path to baseline snapshot JSON")
	newPath := flag.String("new", "", "Path to new snapshot JSON (optional if using fixture)")
	fixturePath := flag.String("fixture", "", "Path to fixture JSON containing XDRs for simulation")
	rpcURL := flag.String("rpc-url", "https://soroban-testnet.stellar.org", "Soroban RPC URL")
	outputJson := flag.String("output-json", "", "Path to write the new snapshot JSON to")
	maxDeltaPct := flag.Float64("max-delta-pct", 10.0, "Maximum allowed percentage increase in resource usage")
	flag.Parse()

	if *baselinePath == "" {
		fmt.Println("Usage: soroban-budget-core --baseline <file> [--new <file> | --fixture <file>]")
		os.Exit(1)
	}

	baselineFile, err := os.Open(*baselinePath)
	if err != nil {
		fmt.Printf("Error opening baseline: %v\n", err)
		os.Exit(1)
	}
	defer baselineFile.Close()

	baseline, err := ParseSnapshot(baselineFile)
	if err != nil {
		exitParseError("baseline", *baselinePath, err)
	}

	var newSnapshot *Snapshot

	if *fixturePath != "" {
		fmt.Printf("Simulating transactions from fixture via %s...\n", *rpcURL)

		fixtureFile, err := os.Open(*fixturePath)
		if err != nil {
			fmt.Printf("Error opening fixture: %v\n", err)
			os.Exit(1)
		}
		defer fixtureFile.Close()

		var fixtures []FixtureEntry
		if err := json.NewDecoder(fixtureFile).Decode(&fixtures); err != nil {
			fmt.Printf("Error decoding fixture JSON: %v\n", err)
			os.Exit(1)
		}

		var funcs []FunctionCost
		for _, f := range fixtures {
			res, simResp, err := MeasureResources(context.Background(), *rpcURL, f.XDR)
			if err != nil {
				fmt.Printf("Failed to measure resources for %s: %v\n", f.Name, err)
				os.Exit(1)
			}

			// Memory is only available from the legacy cost field; record it as unknown otherwise.
			var memBytes *int64
			if simResp.Result.Cost != nil {
				memBytes, err = parseOptionalInt(simResp.Result.Cost.MemoryBytes)
				if err != nil {
					fmt.Printf("Invalid memBytes for %s: %v\n", f.Name, err)
					os.Exit(1)
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
		newFile, err := os.Open(*newPath)
		if err != nil {
			fmt.Printf("Error opening new snapshot: %v\n", err)
			os.Exit(1)
		}
		defer newFile.Close()

		newSnapshot, err = ParseSnapshot(newFile)
		if err != nil {
			exitParseError("new snapshot", *newPath, err)
		}
	} else {
		fmt.Println("Must provide either --new or --fixture.")
		os.Exit(1)
	}

	thresholds := Thresholds{MaxDeltaPct: *maxDeltaPct}
	results := CompareSnapshots(baseline, newSnapshot, thresholds)

	markdown := FormatDiffs(results)
	fmt.Println(markdown)

	if *outputJson != "" {
		data, err := json.MarshalIndent(newSnapshot, "", "  ")
		if err == nil {
			_ = os.WriteFile(*outputJson, data, 0644)
		}
	}

	for _, d := range results {
		if d.Regression {
			os.Exit(1)
		}
	}
}

// exitParseError reports a snapshot that could not be read. An unsupported schema version
// exits 2 on stderr so it is not mistaken for a regression; other parse errors keep the
// existing exit 1 on stdout until exit codes and output streams are reworked (C4, C9).
func exitParseError(label, path string, err error) {
	if errors.Is(err, ErrUnsupportedSchema) {
		fmt.Fprintf(os.Stderr, "Error reading %s %s: %v\n", label, path, err)
		os.Exit(2)
	}
	fmt.Printf("Error parsing %s: %v\n", label, err)
	os.Exit(1)
}
