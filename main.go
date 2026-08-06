package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	baselinePath := flag.String("baseline", "", "Path to baseline snapshot JSON")
	newPath := flag.String("new", "", "Path to new snapshot JSON")
	flag.Parse()

	if *baselinePath == "" || *newPath == "" {
		fmt.Println("Usage: soroban-budget-core --baseline <file> --new <file>")
		os.Exit(1)
	}

	baselineFile, err := os.Open(*baselinePath)
	if err != nil {
		fmt.Printf("Error opening baseline: %v\n", err)
		os.Exit(1)
	}
	defer baselineFile.Close()

	newFile, err := os.Open(*newPath)
	if err != nil {
		fmt.Printf("Error opening new snapshot: %v\n", err)
		os.Exit(1)
	}
	defer newFile.Close()

	baseline, err := ParseSnapshot(baselineFile)
	if err != nil {
		fmt.Printf("Error parsing baseline: %v\n", err)
		os.Exit(1)
	}

	newSnapshot, err := ParseSnapshot(newFile)
	if err != nil {
		fmt.Printf("Error parsing new snapshot: %v\n", err)
		os.Exit(1)
	}

	thresholds := Thresholds{MaxDeltaPct: 5.0} // 5% default threshold
	results := CompareSnapshots(baseline, newSnapshot, thresholds)

	markdown := FormatDiffs(results)
	fmt.Println(markdown)
}
