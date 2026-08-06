package main

import (
	"fmt"
)

type Thresholds struct {
	MaxDeltaPct float64
	MaxAbsolute int64
}

type DiffResult struct {
	FunctionName  string
	Metric        string
	BaselineValue int64
	NewValue      int64
	Delta         int64
	DeltaPct      float64
	Regression    bool
}

// CompareSnapshots compares a baseline snapshot to a new snapshot based on thresholds
func CompareSnapshots(baseline, newSnapshot *Snapshot, thresholds Thresholds) []DiffResult {
	var results []DiffResult

	baselineMap := make(map[string]FunctionCost)
	for _, fn := range baseline.Functions {
		baselineMap[fn.Name] = fn
	}

	for _, newFn := range newSnapshot.Functions {
		if baseFn, exists := baselineMap[newFn.Name]; exists {
			// Compare CPU
			results = append(results, compareMetric(newFn.Name, "CPUInstructions", baseFn.CPUInstructions, newFn.CPUInstructions, thresholds))
			// Compare Memory
			results = append(results, compareMetric(newFn.Name, "MemoryBytes", baseFn.MemoryBytes, newFn.MemoryBytes, thresholds))
			// Compare Read Bytes
			results = append(results, compareMetric(newFn.Name, "ReadBytes", baseFn.ReadBytes, newFn.ReadBytes, thresholds))
			// Compare Write Bytes
			results = append(results, compareMetric(newFn.Name, "WriteBytes", baseFn.WriteBytes, newFn.WriteBytes, thresholds))
		}
	}

	return results
}

func compareMetric(funcName, metric string, baseVal, newVal int64, thresholds Thresholds) DiffResult {
	delta := newVal - baseVal
	var deltaPct float64
	if baseVal > 0 {
		deltaPct = (float64(delta) / float64(baseVal)) * 100.0
	} else if delta > 0 {
		deltaPct = 100.0
	}

	regression := false
	if thresholds.MaxDeltaPct > 0 && deltaPct > thresholds.MaxDeltaPct {
		regression = true
	}
	if thresholds.MaxAbsolute > 0 && delta > thresholds.MaxAbsolute {
		regression = true
	}

	return DiffResult{
		FunctionName:  funcName,
		Metric:        metric,
		BaselineValue: baseVal,
		NewValue:      newVal,
		Delta:         delta,
		DeltaPct:      deltaPct,
		Regression:    regression,
	}
}

// FormatDiffs formats the diff results into a markdown table
func FormatDiffs(results []DiffResult) string {
	markdown := "| Function | Metric | Baseline | New | Delta | % Change | Status |\n"
	markdown += "|----------|--------|----------|-----|-------|----------|--------|\n"

	for _, res := range results {
		status := "✅ OK"
		if res.Regression {
			status = "❌ REGRESSION"
		}
		
		markdown += fmt.Sprintf("| %s | %s | %d | %d | %d | %.2f%% | %s |\n",
			res.FunctionName, res.Metric, res.BaselineValue, res.NewValue, res.Delta, res.DeltaPct, status)
	}

	return markdown
}
