package main

import (
	"fmt"
)

// Thresholds decide which metric changes are regressions or cap warnings. A zero value
// disables that check.
type Thresholds struct {
	MaxDeltaPct   float64          // regression when a metric rises by more than this percentage
	CapWarningPct float64          // warning when a metric exceeds this percentage of its cap
	MaxIncrease   map[string]int64 // per metric name: regression when it rises by more than this
	NetworkCaps   map[string]int64 // per metric name: the network's per-transaction limit; nil when not read
}

type DiffResult struct {
	FunctionName   string
	Metric         string
	BaselineValue  int64
	NewValue       int64
	Delta          int64
	DeltaPct       float64
	AbsoluteCapPct float64
	HasCap         bool // the network cap for this metric is known, so AbsoluteCapPct is meaningful
	Regression     bool
	CapWarning     bool
	Unavailable    bool // metric missing from the baseline or the new snapshot
	Added          bool // function is not in the baseline; only NewValue and the cap figures apply
	Removed        bool // function is in the baseline but not in the new snapshot; one row, no metric
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
		capPct, hasCap, capWarning := capUsage(v.metric, *v.value, thresholds)
		results = append(results, DiffResult{
			FunctionName:   fn.Name,
			Metric:         v.metric,
			NewValue:       *v.value,
			AbsoluteCapPct: capPct,
			HasCap:         hasCap,
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
	if limit := thresholds.MaxIncrease[metric]; limit > 0 && delta > limit {
		regression = true
	}

	absCapPct, hasCap, capWarning := capUsage(metric, newVal, thresholds)

	return DiffResult{
		FunctionName:   funcName,
		Metric:         metric,
		BaselineValue:  baseVal,
		NewValue:       newVal,
		Delta:          delta,
		DeltaPct:       deltaPct,
		AbsoluteCapPct: absCapPct,
		HasCap:         hasCap,
		Regression:     regression,
		CapWarning:     capWarning,
	}
}

// capUsage returns value as a percentage of the metric's network cap, whether that cap is known,
// and whether the percentage is high enough to warn about.
func capUsage(metric string, value int64, thresholds Thresholds) (pct float64, hasCap, warning bool) {
	capLimit := thresholds.NetworkCaps[metric]
	if capLimit <= 0 {
		return 0, false, false
	}
	pct = (float64(value) / float64(capLimit)) * 100.0
	return pct, true, thresholds.CapWarningPct > 0 && pct > thresholds.CapWarningPct
}

// pctOfNetworkCap is the highest share of any metric's per-transaction cap that fn uses: how
// close it is to the limit that would make the transaction fail.
func pctOfNetworkCap(fn FunctionCost, caps map[string]int64) float64 {
	values := map[string]*int64{
		"CPUInstructions": &fn.CPUInstructions,
		"ReadBytes":       &fn.ReadBytes,
		"WriteBytes":      &fn.WriteBytes,
		"ReadEntries":     fn.ReadEntries,
		"WriteEntries":    fn.WriteEntries,
	}
	var highest float64
	for metric, v := range values {
		if v == nil || caps[metric] <= 0 {
			continue
		}
		highest = max(highest, float64(*v)/float64(caps[metric])*100.0)
	}
	return highest
}

// formatCapPct renders the % of Cap column: n/a when the network cap was not read.
func formatCapPct(res DiffResult) string {
	if !res.HasCap {
		return "n/a"
	}
	return fmt.Sprintf("%.2f%%", res.AbsoluteCapPct)
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
			markdown += fmt.Sprintf("| %s | %s | n/a | %d | n/a | n/a | %s | %s |\n",
				res.FunctionName, res.Metric, res.NewValue, formatCapPct(res), status)
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

		markdown += fmt.Sprintf("| %s | %s | %d | %d | %d | %.2f%% | %s | %s |\n",
			res.FunctionName, res.Metric, res.BaselineValue, res.NewValue, res.Delta, res.DeltaPct, formatCapPct(res), status)
	}

	return markdown
}
