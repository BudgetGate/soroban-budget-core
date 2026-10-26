package main

import (
	"fmt"
)

type Thresholds struct {
	MaxDeltaPct float64
	MaxAbsolute int64
}

type DiffResult struct {
	FunctionName   string
	Metric         string
	BaselineValue  int64
	NewValue       int64
	Delta          int64
	DeltaPct       float64
	AbsoluteCapPct float64
	Regression     bool
	CapWarning     bool
	Unavailable    bool // metric missing from the baseline or the new snapshot
	Added          bool // function is not in the baseline; only NewValue and the cap figures apply
	Removed        bool // function is in the baseline but not in the new snapshot; one row, no metric
}

func getCap(metric string) int64 {
	switch metric {
	case "CPUInstructions":
		return 100_000_000
	case "ReadBytes":
		return 10_000_000
	case "WriteBytes":
		return 1_000_000
	}
	return 0
}

func CompareSnapshots(baseline, newSnapshot *Snapshot, thresholds Thresholds) []DiffResult {
	var results []DiffResult

	baselineMap := make(map[string]FunctionCost)
	for _, fn := range baseline.Functions {
		baselineMap[fn.Name] = fn
	}

	inNew := make(map[string]bool)
	for _, newFn := range newSnapshot.Functions {
		inNew[newFn.Name] = true
		baseFn, exists := baselineMap[newFn.Name]
		if !exists {
			results = append(results, addedFunction(newFn, thresholds)...)
			continue
		}
		// MemoryBytes is not compared: current RPCs do not report it, so it would only ever
		// show as unavailable. It is still recorded in snapshots when an RPC provides it.
		results = append(results, compareMetric(newFn.Name, "CPUInstructions", baseFn.CPUInstructions, newFn.CPUInstructions, thresholds))
		results = append(results, compareMetric(newFn.Name, "ReadBytes", baseFn.ReadBytes, newFn.ReadBytes, thresholds))
		results = append(results, compareMetric(newFn.Name, "WriteBytes", baseFn.WriteBytes, newFn.WriteBytes, thresholds))
		results = append(results, compareOptionalMetric(newFn.Name, "ReadEntries", baseFn.ReadEntries, newFn.ReadEntries, thresholds))
		results = append(results, compareOptionalMetric(newFn.Name, "WriteEntries", baseFn.WriteEntries, newFn.WriteEntries, thresholds))
	}

	// Listing functions that disappeared makes a removal or a rename visible.
	for _, baseFn := range baseline.Functions {
		if !inNew[baseFn.Name] {
			results = append(results, DiffResult{FunctionName: baseFn.Name, Removed: true})
		}
	}

	return results
}

// addedFunction reports a function that has no baseline: its values and cap usage, never a
// regression, since there is nothing to compare against.
func addedFunction(fn FunctionCost, thresholds Thresholds) []DiffResult {
	values := []struct {
		metric string
		value  *int64
	}{
		{"CPUInstructions", &fn.CPUInstructions},
		{"ReadBytes", &fn.ReadBytes},
		{"WriteBytes", &fn.WriteBytes},
		{"ReadEntries", fn.ReadEntries},
		{"WriteEntries", fn.WriteEntries},
	}

	results := make([]DiffResult, 0, len(values))
	for _, v := range values {
		if v.value == nil {
			results = append(results, DiffResult{FunctionName: fn.Name, Metric: v.metric, Added: true, Unavailable: true})
			continue
		}
		capPct, capWarning := capUsage(v.metric, *v.value, thresholds)
		results = append(results, DiffResult{
			FunctionName:   fn.Name,
			Metric:         v.metric,
			NewValue:       *v.value,
			AbsoluteCapPct: capPct,
			CapWarning:     capWarning,
			Added:          true,
		})
	}
	return results
}

func compareOptionalMetric(funcName, metric string, baseVal, newVal *int64, thresholds Thresholds) DiffResult {
	if baseVal == nil || newVal == nil {
		return DiffResult{FunctionName: funcName, Metric: metric, Unavailable: true}
	}
	return compareMetric(funcName, metric, *baseVal, *newVal, thresholds)
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

	absCapPct, capWarning := capUsage(metric, newVal, thresholds)

	return DiffResult{
		FunctionName:   funcName,
		Metric:         metric,
		BaselineValue:  baseVal,
		NewValue:       newVal,
		Delta:          delta,
		DeltaPct:       deltaPct,
		AbsoluteCapPct: absCapPct,
		Regression:     regression,
		CapWarning:     capWarning,
	}
}

// capUsage returns value as a percentage of the metric's cap, and whether that is high enough
// to warn about. Metrics without a cap return 0, false.
func capUsage(metric string, value int64, thresholds Thresholds) (float64, bool) {
	capLimit := getCap(metric)
	if capLimit <= 0 {
		return 0, false
	}
	pct := (float64(value) / float64(capLimit)) * 100.0
	return pct, pct > 80.0 // 80% of network cap is a warning
}

func FormatDiffs(results []DiffResult) string {
	markdown := "| Function | Metric | Baseline | New | Delta | % Change | % of Cap | Status |\n"
	markdown += "|----------|--------|----------|-----|-------|----------|----------|--------|\n"

	for _, res := range results {
		if res.Removed {
			markdown += fmt.Sprintf("| %s | — | n/a | n/a | n/a | n/a | n/a | 🗑️ REMOVED |\n", res.FunctionName)
			continue
		}
		if res.Unavailable {
			markdown += fmt.Sprintf("| %s | %s | n/a | n/a | n/a | n/a | n/a | ➖ NOT REPORTED |\n", res.FunctionName, res.Metric)
			continue
		}
		if res.Added {
			status := "🆕 NEW"
			if res.CapWarning {
				status = "🆕 NEW & ⚠️ CAP WARNING"
			}
			markdown += fmt.Sprintf("| %s | %s | n/a | %d | n/a | n/a | %.2f%% | %s |\n",
				res.FunctionName, res.Metric, res.NewValue, res.AbsoluteCapPct, status)
			continue
		}

		status := "✅ OK"
		if res.Regression && res.CapWarning {
			status = "❌ REGRESSION & ⚠️ CAP WARNING"
		} else if res.Regression {
			status = "❌ REGRESSION"
		} else if res.CapWarning {
			status = "⚠️ CAP WARNING"
		}

		markdown += fmt.Sprintf("| %s | %s | %d | %d | %d | %.2f%% | %.2f%% | %s |\n",
			res.FunctionName, res.Metric, res.BaselineValue, res.NewValue, res.Delta, res.DeltaPct, res.AbsoluteCapPct, status)
	}

	return markdown
}
