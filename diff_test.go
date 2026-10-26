package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCompareSnapshots(t *testing.T) {
	baseline := &Snapshot{
		Functions: []FunctionCost{
			{Name: "mint", CPUInstructions: 1000, MemoryBytes: int64Ptr(500)},
		},
	}

	newSnapshot := &Snapshot{
		Functions: []FunctionCost{
			{Name: "mint", CPUInstructions: 1100, MemoryBytes: int64Ptr(500)},
		},
	}

	thresholds := Thresholds{MaxDeltaPct: 5.0}

	results := CompareSnapshots(baseline, newSnapshot, thresholds)

	assert.Len(t, results, 5) // CPU, ReadBytes, WriteBytes, ReadEntries, WriteEntries

	// CPU Should regress (10% increase > 5% threshold)
	assert.Equal(t, "CPUInstructions", results[0].Metric)
	assert.Equal(t, int64(100), results[0].Delta)
	assert.Equal(t, 10.0, results[0].DeltaPct)
	assert.True(t, results[0].Regression)

	assert.Equal(t, "ReadBytes", results[1].Metric)
	assert.False(t, results[1].Regression)
}

func TestCompareSnapshots_MemoryNotInReport(t *testing.T) {
	// Memory is excluded from the report even when both snapshots carry a value
	// and it regressed, until a real source for it exists.
	baseline := &Snapshot{
		Functions: []FunctionCost{
			{Name: "mint", CPUInstructions: 1000, MemoryBytes: int64Ptr(500)},
		},
	}

	newSnapshot := &Snapshot{
		Functions: []FunctionCost{
			{Name: "mint", CPUInstructions: 1000, MemoryBytes: int64Ptr(5000)},
		},
	}

	results := CompareSnapshots(baseline, newSnapshot, Thresholds{MaxDeltaPct: 5.0})

	for _, r := range results {
		assert.NotEqual(t, "MemoryBytes", r.Metric)
		assert.False(t, r.Regression)
	}
	assert.NotContains(t, FormatDiffs(results), "MemoryBytes")
}

func TestCompareSnapshots_FootprintEntries(t *testing.T) {
	baseline := &Snapshot{
		Functions: []FunctionCost{
			{Name: "transfer", ReadEntries: int64Ptr(3), WriteEntries: int64Ptr(2)},
		},
	}

	// One extra contract-storage read: invisible to ReadBytes, caught by ReadEntries.
	newSnapshot := &Snapshot{
		Functions: []FunctionCost{
			{Name: "transfer", ReadEntries: int64Ptr(4), WriteEntries: int64Ptr(2)},
		},
	}

	results := CompareSnapshots(baseline, newSnapshot, Thresholds{MaxDeltaPct: 10.0})

	assert.Equal(t, "ReadEntries", results[3].Metric)
	assert.True(t, results[3].Regression)
	assert.Equal(t, "WriteEntries", results[4].Metric)
	assert.False(t, results[4].Regression)
}

func TestCompareSnapshots_FootprintEntriesMissingFromBaseline(t *testing.T) {
	// Baselines written before entry counts existed must not report a regression.
	baseline := &Snapshot{Functions: []FunctionCost{{Name: "transfer"}}}
	newSnapshot := &Snapshot{
		Functions: []FunctionCost{
			{Name: "transfer", ReadEntries: int64Ptr(3), WriteEntries: int64Ptr(2)},
		},
	}

	results := CompareSnapshots(baseline, newSnapshot, Thresholds{MaxDeltaPct: 10.0})

	assert.True(t, results[3].Unavailable)
	assert.True(t, results[4].Unavailable)
	assert.False(t, results[3].Regression || results[4].Regression)
}

func int64Ptr(v int64) *int64 {
	return &v
}

func TestFormatDiffs(t *testing.T) {
	results := []DiffResult{
		{FunctionName: "mint", Metric: "CPU", BaselineValue: 1000, NewValue: 1100, Delta: 100, DeltaPct: 10.0, Regression: true},
	}

	markdown := FormatDiffs(results)
	assert.Contains(t, markdown, "| mint | CPU | 1000 | 1100 | 100 | 10.00% | 0.00% | ❌ REGRESSION |")
}
