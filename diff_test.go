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
		{
			name: "added",
			res:  DiffResult{FunctionName: "burn", Metric: "CPUInstructions", NewValue: 91406, AbsoluteCapPct: 0.09, Added: true},
			want: "| burn | CPUInstructions | n/a | 91406 | n/a | n/a | 0.09% | 🆕 NEW |",
		},
		{
			name: "added near the cap",
			res:  DiffResult{FunctionName: "burn", Metric: "CPUInstructions", NewValue: 90000000, AbsoluteCapPct: 90, CapWarning: true, Added: true},
			want: "| burn | CPUInstructions | n/a | 90000000 | n/a | n/a | 90.00% | 🆕 NEW & ⚠️ CAP WARNING |",
		},
		{
			name: "removed",
			res:  DiffResult{FunctionName: "mint", Removed: true},
			want: "| mint | — | n/a | n/a | n/a | n/a | n/a | 🗑️ REMOVED |",
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

func TestCompareSnapshots_AddedRemoved(t *testing.T) {
	fn := func(name string, cpu int64) FunctionCost {
		return FunctionCost{Name: name, CPUInstructions: cpu, ReadBytes: 144, WriteBytes: 368, ReadEntries: int64Ptr(3), WriteEntries: int64Ptr(2)}
	}

	type row struct {
		function string
		metric   string
		added    bool
		removed  bool
	}
	tests := []struct {
		name     string
		baseline []FunctionCost
		newFns   []FunctionCost
		want     []row // in report order
	}{
		{
			name:     "function added",
			baseline: []FunctionCost{fn("f", 1000)},
			newFns:   []FunctionCost{fn("f", 1000), fn("g", 5000)},
			want: []row{
				{"f", "CPUInstructions", false, false}, {"f", "ReadBytes", false, false}, {"f", "WriteBytes", false, false}, {"f", "ReadEntries", false, false}, {"f", "WriteEntries", false, false},
				{"g", "CPUInstructions", true, false}, {"g", "ReadBytes", true, false}, {"g", "WriteBytes", true, false}, {"g", "ReadEntries", true, false}, {"g", "WriteEntries", true, false},
			},
		},
		{
			name:     "function removed",
			baseline: []FunctionCost{fn("f", 1000), fn("g", 5000)},
			newFns:   []FunctionCost{fn("f", 1000)},
			want: []row{
				{"f", "CPUInstructions", false, false}, {"f", "ReadBytes", false, false}, {"f", "WriteBytes", false, false}, {"f", "ReadEntries", false, false}, {"f", "WriteEntries", false, false},
				{"g", "", false, true},
			},
		},
		{
			// A rename shows as one added and one removed function, so it cannot pass unnoticed.
			name:     "function renamed",
			baseline: []FunctionCost{fn("f", 1000)},
			newFns:   []FunctionCost{fn("f2", 1000)},
			want: []row{
				{"f2", "CPUInstructions", true, false}, {"f2", "ReadBytes", true, false}, {"f2", "WriteBytes", true, false}, {"f2", "ReadEntries", true, false}, {"f2", "WriteEntries", true, false},
				{"f", "", false, true},
			},
		},
		{
			name:     "empty baseline",
			baseline: nil,
			newFns:   []FunctionCost{fn("f", 1000)},
			want: []row{
				{"f", "CPUInstructions", true, false}, {"f", "ReadBytes", true, false}, {"f", "WriteBytes", true, false}, {"f", "ReadEntries", true, false}, {"f", "WriteEntries", true, false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := CompareSnapshots(&Snapshot{Functions: tt.baseline}, &Snapshot{Functions: tt.newFns}, Thresholds{MaxDeltaPct: 10})
			var got []row
			for _, r := range results {
				got = append(got, row{r.FunctionName, r.Metric, r.Added, r.Removed})
				// Neither an added nor a removed function is a regression.
				if r.Added || r.Removed {
					assert.False(t, r.Regression, "%s %s", r.FunctionName, r.Metric)
				}
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompareSnapshots_AddedFunctionValues(t *testing.T) {
	// 90M of the 100M CPU cap: an added function still gets a cap warning.
	added := FunctionCost{Name: "g", CPUInstructions: 90_000_000, ReadBytes: 144, WriteBytes: 0}
	results := CompareSnapshots(&Snapshot{}, &Snapshot{Functions: []FunctionCost{added}}, Thresholds{MaxDeltaPct: 10})

	require.Len(t, results, 5)
	assert.Equal(t, int64(90_000_000), results[0].NewValue)
	assert.InDelta(t, 90.0, results[0].AbsoluteCapPct, 1e-9)
	assert.True(t, results[0].CapWarning)
	assert.False(t, results[1].CapWarning)
	// Entry counts missing from the new snapshot (e.g. a 1.0 file) are reported as unavailable.
	assert.True(t, results[3].Unavailable && results[3].Added)
	assert.True(t, results[4].Unavailable && results[4].Added)
}
