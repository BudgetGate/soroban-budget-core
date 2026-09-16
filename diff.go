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
}

func getCap(metric string) int64 {
	switch metric {
	case "CPUInstructions":
		return 100_000_000
	case "MemoryBytes":
		return 40_000_000
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

	for _, newFn := range newSnapshot.Functions {
		if baseFn, exists := baselineMap[newFn.Name]; exists {
			results = append(results, compareMetric(newFn.Name, "CPUInstructions", baseFn.CPUInstructions, newFn.CPUInstructions, thresholds))
			results = append(results, compareMetric(newFn.Name, "MemoryBytes", baseFn.MemoryBytes, newFn.MemoryBytes, thresholds))
			results = append(results, compareMetric(newFn.Name, "ReadBytes", baseFn.ReadBytes, newFn.ReadBytes, thresholds))
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

	capLimit := getCap(metric)
	var absCapPct float64
	capWarning := false
	if capLimit > 0 {
		absCapPct = (float64(newVal) / float64(capLimit)) * 100.0
		if absCapPct > 80.0 { // 80% of network cap is a warning
			capWarning = true
		}
	}

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

func FormatDiffs(results []DiffResult) string {
	markdown := "| Function | Metric | Baseline | New | Delta | % Change | % of Cap | Status |\n"
	markdown += "|----------|--------|----------|-----|-------|----------|----------|--------|\n"

	for _, res := range results {
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
