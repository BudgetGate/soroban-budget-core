package main

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestCompareSnapshots(t *testing.T) {
	baseline := &Snapshot{
		Functions: []FunctionCost{
			{Name: "mint", CPUInstructions: 1000, MemoryBytes: 500},
		},
	}

	newSnapshot := &Snapshot{
		Functions: []FunctionCost{
			{Name: "mint", CPUInstructions: 1100, MemoryBytes: 500},
		},
	}

	thresholds := Thresholds{MaxDeltaPct: 5.0}

	results := CompareSnapshots(baseline, newSnapshot, thresholds)

	assert.Len(t, results, 4) // CPU, Mem, Read, Write
	
	// CPU Should regress (10% increase > 5% threshold)
	assert.Equal(t, "CPUInstructions", results[0].Metric)
	assert.Equal(t, int64(100), results[0].Delta)
	assert.Equal(t, 10.0, results[0].DeltaPct)
	assert.True(t, results[0].Regression)

	// Memory Should not regress (0% increase)
	assert.Equal(t, "MemoryBytes", results[1].Metric)
	assert.False(t, results[1].Regression)
}

func TestFormatDiffs(t *testing.T) {
	results := []DiffResult{
		{FunctionName: "mint", Metric: "CPU", BaselineValue: 1000, NewValue: 1100, Delta: 100, DeltaPct: 10.0, Regression: true},
	}
	
	markdown := FormatDiffs(results)
	assert.Contains(t, markdown, "| mint | CPU | 1000 | 1100 | 100 | 10.00% | 0.00% | ❌ REGRESSION |")
}
