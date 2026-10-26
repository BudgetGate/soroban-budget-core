package main

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func int64Ptr(v int64) *int64 {
	return &v
}

func TestCompareSnapshots(t *testing.T) {
	// base has every compared metric set, so a case only lists the metrics it expects to flag.
	base := FunctionCost{
		Name:            "transfer",
		CPUInstructions: 1000,
		MemoryBytes:     int64Ptr(500),
		ReadBytes:       144,
		WriteBytes:      368,
		ReadEntries:     int64Ptr(3),
		WriteEntries:    int64Ptr(2),
	}

	type flags struct{ regression, unavailable bool }
	tests := []struct {
		name         string
		changeBase   func(fc *FunctionCost)
		changeNew    func(fc *FunctionCost)
		maxDeltaPct  float64
		want         map[string]flags // metrics not listed must be neither
		wantCPUDelta float64
	}{
		{
			name:         "CPU rise above threshold",
			changeNew:    func(fc *FunctionCost) { fc.CPUInstructions = 1100 },
			maxDeltaPct:  5,
			want:         map[string]flags{"CPUInstructions": {regression: true}},
			wantCPUDelta: 10,
		},
		{
			name:         "CPU rise within threshold",
			changeNew:    func(fc *FunctionCost) { fc.CPUInstructions = 1040 },
			maxDeltaPct:  5,
			wantCPUDelta: 4,
		},
		{
			name:         "CPU drop",
			changeNew:    func(fc *FunctionCost) { fc.CPUInstructions = 500 },
			maxDeltaPct:  5,
			wantCPUDelta: -50,
		},
		{
			// Memory is excluded even when both snapshots carry a value and it regressed,
			// until a real source for it exists.
			name:        "memory is not compared",
			changeNew:   func(fc *FunctionCost) { fc.MemoryBytes = int64Ptr(5000) },
			maxDeltaPct: 5,
		},
		{
			// One extra contract-storage read: invisible to ReadBytes, caught by ReadEntries.
			name:        "extra storage read",
			changeNew:   func(fc *FunctionCost) { fc.ReadEntries = int64Ptr(4) },
			maxDeltaPct: 10,
			want:        map[string]flags{"ReadEntries": {regression: true}},
		},
		{
			// A zero baseline that rises counts as a 100% increase.
			name:        "first write from zero",
			changeBase:  func(fc *FunctionCost) { fc.WriteBytes = 0 },
			changeNew:   func(fc *FunctionCost) { fc.WriteBytes = 52 },
			maxDeltaPct: 10,
			want:        map[string]flags{"WriteBytes": {regression: true}},
		},
		{
			// Baselines written before entry counts existed must not report a regression.
			name:        "entry counts missing from baseline",
			changeBase:  func(fc *FunctionCost) { fc.ReadEntries, fc.WriteEntries = nil, nil },
			maxDeltaPct: 10,
			want:        map[string]flags{"ReadEntries": {unavailable: true}, "WriteEntries": {unavailable: true}},
		},
		{
			name:         "threshold 0 disables the delta check",
			changeNew:    func(fc *FunctionCost) { fc.CPUInstructions = 5000 },
			maxDeltaPct:  0,
			wantCPUDelta: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseFn, newFn := base, base
			if tt.changeBase != nil {
				tt.changeBase(&baseFn)
			}
			if tt.changeNew != nil {
				tt.changeNew(&newFn)
			}

			results := CompareSnapshots(
				&Snapshot{Functions: []FunctionCost{baseFn}},
				&Snapshot{Functions: []FunctionCost{newFn}},
				Thresholds{MaxDeltaPct: tt.maxDeltaPct},
			)

			var metrics []string
			for _, r := range results {
				metrics = append(metrics, r.Metric)
				want := tt.want[r.Metric]
				assert.Equal(t, want.regression, r.Regression, "%s regression", r.Metric)
				assert.Equal(t, want.unavailable, r.Unavailable, "%s unavailable", r.Metric)
			}
			assert.Equal(t, []string{"CPUInstructions", "ReadBytes", "WriteBytes", "ReadEntries", "WriteEntries"}, metrics)
			require.NotEmpty(t, results)
			assert.InDelta(t, tt.wantCPUDelta, results[0].DeltaPct, 1e-9)
		})
	}
}

func TestFormatDiffs(t *testing.T) {
	tests := []struct {
		name string
		res  DiffResult
		want string
	}{
		{
			name: "regression",
			res:  DiffResult{FunctionName: "mint", Metric: "CPUInstructions", BaselineValue: 1000, NewValue: 1100, Delta: 100, DeltaPct: 10, Regression: true},
			want: "| mint | CPUInstructions | 1000 | 1100 | 100 | 10.00% | 0.00% | ❌ REGRESSION |",
		},
		{
			name: "ok",
			res:  DiffResult{FunctionName: "mint", Metric: "ReadBytes", BaselineValue: 144, NewValue: 144, AbsoluteCapPct: 0.5},
			want: "| mint | ReadBytes | 144 | 144 | 0 | 0.00% | 0.50% | ✅ OK |",
		},
		{
			name: "cap warning",
			res:  DiffResult{FunctionName: "mint", Metric: "CPUInstructions", BaselineValue: 85, NewValue: 85, AbsoluteCapPct: 85, CapWarning: true},
			want: "| mint | CPUInstructions | 85 | 85 | 0 | 0.00% | 85.00% | ⚠️ CAP WARNING |",
		},
		{
			name: "regression and cap warning",
			res:  DiffResult{FunctionName: "mint", Metric: "CPUInstructions", BaselineValue: 50, NewValue: 90, Delta: 40, DeltaPct: 80, AbsoluteCapPct: 90, Regression: true, CapWarning: true},
			want: "| mint | CPUInstructions | 50 | 90 | 40 | 80.00% | 90.00% | ❌ REGRESSION & ⚠️ CAP WARNING |",
		},
		{
			name: "unavailable",
			res:  DiffResult{FunctionName: "mint", Metric: "ReadEntries", Unavailable: true},
			want: "| mint | ReadEntries | n/a | n/a | n/a | n/a | n/a | ➖ NOT REPORTED |",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			markdown := FormatDiffs([]DiffResult{tt.res})
			assert.Contains(t, markdown, "| Function | Metric | Baseline | New | Delta | % Change | % of Cap | Status |")
			assert.Contains(t, markdown, tt.want)
		})
	}
}
