package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"
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
	commitSHA := fs.String("commit-sha", "", "Commit of the contract code being measured, recorded in the snapshot (fixture runs)")
	maxDeltaPct := fs.Float64("max-delta-pct", 10.0, "Maximum allowed percentage increase in resource usage (0 disables)")
	capWarningPct := fs.Float64("cap-warning-pct", 80.0, "Warn when a metric exceeds this percentage of its network cap (0 disables)")
	failOnCapWarning := fs.Bool("fail-on-cap-warning", false, "Exit 1 when any metric has a cap warning")
	maxIncrease := map[string]*int64{
		"CPUInstructions": fs.Int64("max-cpu-increase", 0, "Regression when CPU instructions rise by more than this (0 disables)"),
		"ReadBytes":       fs.Int64("max-read-bytes-increase", 0, "Regression when read bytes rise by more than this (0 disables)"),
		"WriteBytes":      fs.Int64("max-write-bytes-increase", 0, "Regression when write bytes rise by more than this (0 disables)"),
		"ReadEntries":     fs.Int64("max-read-entries-increase", 0, "Regression when read entries rise by more than this (0 disables)"),
		"WriteEntries":    fs.Int64("max-write-entries-increase", 0, "Regression when write entries rise by more than this (0 disables)"),
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitError
	}

	thresholds := Thresholds{MaxDeltaPct: *maxDeltaPct, CapWarningPct: *capWarningPct, MaxIncrease: map[string]int64{}}
	if *maxDeltaPct < 0 || *capWarningPct < 0 {
		fmt.Fprintln(stderr, "Thresholds must not be negative.")
		return exitError
	}
	for metric, v := range maxIncrease {
		if *v < 0 {
			fmt.Fprintln(stderr, "Thresholds must not be negative.")
			return exitError
		}
		thresholds.MaxIncrease[metric] = *v
	}
	if *failOnCapWarning && *capWarningPct == 0 {
		fmt.Fprintln(stderr, "--fail-on-cap-warning needs --cap-warning-pct above 0.")
		return exitError
	}

	if *baselinePath == "" {
		fmt.Fprintln(stderr, "Usage: soroban-budget-core --baseline <file> [--new <file> | --fixture <file>]")
		return exitError
	}
	if *fixturePath == "" && *newPath == "" {
		fmt.Fprintln(stderr, "Must provide either --new or --fixture.")
		return exitError
	}

	baseline, code := readSnapshot("baseline", *baselinePath, stderr)
	if code != exitOK {
		return code
	}
	// A baseline written before names had to be unique may repeat one. Comparing against either
	// entry would be a guess, so those functions get no baseline and show as new; the next
	// baseline saved from this run no longer has the duplicate.
	if dups := duplicateNames(baseline.Functions, functionName); len(dups) > 0 {
		fmt.Fprintf(stderr, "Warning: the baseline has more than one function named %s; comparing them as new functions.\n", quotedList(dups))
		baseline.Functions = withoutNames(baseline.Functions, dups)
	}

	// Caps are read live so they match the network and its current protocol version; there is no
	// built-in fallback. A fixture run needs the network anyway. A --new run skips the lookup when
	// cap warnings are off, so it can work offline; % of Cap then shows n/a.
	var caps NetworkCaps
	if *fixturePath != "" || *capWarningPct > 0 {
		fmt.Fprintf(stderr, "Reading network caps from %s...\n", *rpcURL)
		var err error
		caps, err = FetchNetworkCaps(context.Background(), *rpcURL)
		if err != nil {
			fmt.Fprintf(stderr, "Error reading network caps from %s: %v\n", *rpcURL, err)
			return exitError
		}
		thresholds.NetworkCaps = caps.Limits
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
		if dups := duplicateNames(fixtures, func(f FixtureEntry) string { return f.Name }); len(dups) > 0 {
			fmt.Fprintf(stderr, "Fixture has more than one entry named %s; each name must be unique, since the baseline is matched by name.\n", quotedList(dups))
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

			fc := FunctionCost{
				Name:            f.Name,
				FixtureID:       f.FixtureID,
				CPUInstructions: res.CPUInstructions,
				MemoryBytes:     memBytes,
				ReadBytes:       res.ReadBytes,
				WriteBytes:      res.WriteBytes,
				ReadEntries:     &res.ReadEntries,
				WriteEntries:    &res.WriteEntries,
			}
			fc.PctOfNetworkCap = pctOfNetworkCap(fc, caps.Limits)
			funcs = append(funcs, fc)
		}

		// A fixture run always reads the network caps, so the network is known here.
		newSnapshot = &Snapshot{
			SchemaVersion: CurrentSchemaVersion,
			Network:       networkName(caps.Passphrase),
			GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
			CommitSHA:     *commitSHA,
			Functions:     funcs,
		}
	} else {
		newSnapshot, code = readSnapshot("new snapshot", *newPath, stderr)
		if code != exitOK {
			return code
		}
		if dups := duplicateNames(newSnapshot.Functions, functionName); len(dups) > 0 {
			fmt.Fprintf(stderr, "New snapshot has more than one function named %s; each name must be unique, since the baseline is matched by name.\n", quotedList(dups))
			return exitError
		}
	}

	results := CompareSnapshots(baseline, newSnapshot, thresholds)

	markdown := FormatDiffs(results)
	fmt.Fprint(stdout, markdown)
	if caps.Limits != nil {
		fmt.Fprintf(stdout, "\n%% of Cap uses the network's per-transaction limits at protocol %d, ledger %d.\n", caps.ProtocolVersion, caps.Ledger)
	}

	// A snapshot that was asked for but not written fails the run, even after a regression: a
	// caller that saves it as the next baseline must not continue with a missing or stale file.
	// The report above still shows any regression.
	if *outputJson != "" {
		if err := writeSnapshot(*outputJson, newSnapshot); err != nil {
			fmt.Fprintf(stderr, "Error writing --output-json %s: %v\n", *outputJson, err)
			return exitError
		}
	}

	capWarnings := 0
	for _, d := range results {
		if d.Regression {
			return exitRegression
		}
		if d.CapWarning {
			capWarnings++
		}
	}
	if *failOnCapWarning && capWarnings > 0 {
		fmt.Fprintf(stderr, "Failing: %d cap warning(s) and --fail-on-cap-warning is set.\n", capWarnings)
		return exitRegression
	}
	return exitOK
}

// writeSnapshot writes s as indented JSON.
func writeSnapshot(path string, s *Snapshot) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
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
