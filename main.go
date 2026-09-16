package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
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
		fmt.Printf("Error parsing baseline: %v\n", err)
		os.Exit(1)
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
			simResp, err := SimulateTransaction(*rpcURL, f.XDR)
			if err != nil {
				fmt.Printf("RPC Error for %s: %v\n", f.Name, err)
				os.Exit(1)
			}

			readBytes, writeBytes, cpuInsns, memBytes, err := ParseTransactionData(simResp.Result.TransactionData)
			if err != nil {
				fmt.Printf("Failed to parse transaction data for %s: %v\n", f.Name, err)
				os.Exit(1)
			}
			
			// Fallback to cost object if script didn't extract them
			if cpuInsns == 0 {
				cpuInsns, _ = strconv.ParseInt(simResp.Result.Cost.CPUInstructions, 10, 64)
			}
			if memBytes == 0 {
				memBytes, _ = strconv.ParseInt(simResp.Result.Cost.MemoryBytes, 10, 64)
			}

			funcs = append(funcs, FunctionCost{
				Name:            f.Name,
				FixtureID:       f.FixtureID,
				CPUInstructions: cpuInsns,
				MemoryBytes:     memBytes,
				ReadBytes:       readBytes,
				WriteBytes:      writeBytes,
			})
		}

		newSnapshot = &Snapshot{
			SchemaVersion: "1.0",
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
			fmt.Printf("Error parsing new snapshot: %v\n", err)
			os.Exit(1)
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
