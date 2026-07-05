package correlation

import (
	"math"
	"testing"
	"time"
)

func TestHayashiYoshidaCorrelation(t *testing.T) {
	left := []Sample{
		{At: time.Unix(0, 0), Value: 100},
		{At: time.Unix(1, 0), Value: 110},
		{At: time.Unix(2, 0), Value: 121},
		{At: time.Unix(3, 0), Value: 133.1},
	}
	right := []Sample{
		{At: time.Unix(0, 0), Value: 50},
		{At: time.Unix(1, 0), Value: 55},
		{At: time.Unix(2, 0), Value: 60.5},
		{At: time.Unix(3, 0), Value: 66.55},
	}

	value, ok := hayashiYoshidaCorrelation(left, right, time.Second)
	if !ok {
		t.Fatal("expected correlation")
	}

	if math.Abs(value-1) > 1e-9 {
		t.Fatalf("correlation = %.12f, want 1", value)
	}
}

func TestGapSegmentsAndScores(t *testing.T) {
	batch := coupledGapBatch()
	syncLeft, syncRight, asyncLeft, asyncRight, ok := gapSegments(batch)
	if !ok {
		t.Fatal("expected segmented gap batch")
	}

	if len(syncLeft) != 6 || len(syncRight) != 6 {
		t.Fatalf("sync segment widths = %d/%d", len(syncLeft), len(syncRight))
	}

	if len(asyncLeft) != 8 || len(asyncRight) != 8 {
		t.Fatalf("async segment widths = %d/%d", len(asyncLeft), len(asyncRight))
	}

	pearson, ok := gapPearson(batch, nil)
	if !ok || pearson <= 0.9 {
		t.Fatalf("pearson = %f/%t, want > 0.9/true", pearson, ok)
	}

	hayashi, ok := gapHayashi(batch, time.Second)
	if !ok || math.Abs(hayashi-1) > 1e-6 {
		t.Fatalf("hayashi = %f/%t, want 1/true", hayashi, ok)
	}
}

func TestIntervalCorrelationSlices(t *testing.T) {
	start := []float64{0, 1, 2}
	end := []float64{1, 2, 3}
	left := []float64{1, 2, 3}
	right := []float64{2, 4, 6}

	value, ok := intervalCorrelationSlices(start, end, left, start, end, right)
	if !ok {
		t.Fatal("expected interval correlation")
	}

	if math.Abs(value-1) > 1e-9 {
		t.Fatalf("interval correlation = %.12f, want 1", value)
	}
}

func TestMedianPairwiseAbsCorrelation(t *testing.T) {
	series := []intervalSlices{
		{
			starts: []float64{0, 1, 2},
			ends:   []float64{1, 2, 3},
			rets:   []float64{1, 2, 3},
		},
		{
			starts: []float64{0, 1, 2},
			ends:   []float64{1, 2, 3},
			rets:   []float64{2, 4, 6},
		},
	}

	value := medianPairwiseAbsCorrelation(series)
	if math.Abs(value-1) > 1e-9 {
		t.Fatalf("median pairwise = %.12f, want 1", value)
	}
}

func coupledGapBatch() []float64 {
	syncLeft := []float64{1, 2, 3, 4, 5, 6}
	syncRight := []float64{2, 4, 6, 8, 10, 12}
	asyncLeft := []float64{0, 100, 1, 110, 2, 121, 3, 133.1}
	asyncRight := []float64{0, 50, 1, 55, 2, 60.5, 3, 66.55}

	batch := make(
		[]float64,
		0,
		2+len(syncLeft)+len(syncRight)+len(asyncLeft)+len(asyncRight),
	)
	batch = append(batch, float64(len(syncLeft)), float64(len(asyncLeft)/2))
	batch = append(batch, syncLeft...)
	batch = append(batch, syncRight...)
	batch = append(batch, asyncLeft...)
	batch = append(batch, asyncRight...)

	return batch
}

func BenchmarkGapPearson(testingTB *testing.B) {
	batch := coupledGapBatch()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		gapPearson(batch, nil)
	}
}
