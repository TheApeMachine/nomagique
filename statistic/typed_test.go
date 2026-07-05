package statistic

import (
	"math"
	"testing"
	"time"

	"gonum.org/v1/gonum/stat"
)

func TestTypedScalarStatistics(t *testing.T) {
	mean := NewMean()
	sum := NewSum()
	minimum := NewMin()
	maximum := NewMax()

	for _, sample := range []float64{1, 2, 3, 4} {
		if _, err := mean.Measure(sample); err != nil {
			t.Fatal(err)
		}

		if _, err := sum.Measure(sample); err != nil {
			t.Fatal(err)
		}

		if _, err := minimum.Measure(sample); err != nil {
			t.Fatal(err)
		}

		if _, err := maximum.Measure(sample); err != nil {
			t.Fatal(err)
		}
	}

	meanOutput, _ := mean.Measure(5)
	sumOutput, _ := sum.Measure(5)
	minOutput, _ := minimum.Measure(5)
	maxOutput, _ := maximum.Measure(5)

	if meanOutput.Value != 3 {
		t.Fatalf("mean = %v, want 3", meanOutput.Value)
	}

	if sumOutput.Value != 15 {
		t.Fatalf("sum = %v, want 15", sumOutput.Value)
	}

	if minOutput.Value != 1 {
		t.Fatalf("min = %v, want 1", minOutput.Value)
	}

	if maxOutput.Value != 5 {
		t.Fatalf("max = %v, want 5", maxOutput.Value)
	}
}

func TestTypedHistoryStatistics(t *testing.T) {
	stdDev := NewStdDev()
	medianAbsolute := NewMedianAbsolute()
	quantile := NewQuantile(QuantileConfig{Percentile: 0.75, Kind: stat.LinInterp})

	for _, sample := range []float64{-1, 2, -3, 4} {
		if _, err := stdDev.Measure(sample); err != nil {
			t.Fatal(err)
		}

		if _, err := medianAbsolute.Measure(sample); err != nil {
			t.Fatal(err)
		}

		if _, err := quantile.Measure(sample); err != nil {
			t.Fatal(err)
		}
	}

	stdOutput, _ := stdDev.Measure(5)
	absoluteOutput, _ := medianAbsolute.Measure(-5)
	quantileOutput, _ := quantile.Measure(5)

	if !stdOutput.Ready || stdOutput.Value <= 0 {
		t.Fatalf("stddev = %+v, want positive ready output", stdOutput)
	}

	if absoluteOutput.Value != 3 {
		t.Fatalf("median absolute = %v, want 3", absoluteOutput.Value)
	}

	if quantileOutput.Value <= 0 {
		t.Fatalf("quantile = %v, want positive value", quantileOutput.Value)
	}
}

func TestTypedPanelAndPeerMedian(t *testing.T) {
	panel := NewPanel()
	_, _ = panel.Measure(PanelSample{Member: "A", Value: 1})
	_, _ = panel.Measure(PanelSample{Member: "B", Value: 3})
	snapshot, err := panel.Measure(PanelSample{Member: "C", Value: 5})
	if err != nil {
		t.Fatal(err)
	}

	median := NewMedian()
	output, err := median.MeasurePeers("C", snapshot.Peers)
	if err != nil {
		t.Fatal(err)
	}

	if output.Value != 2 {
		t.Fatalf("peer median = %v, want 2", output.Value)
	}
}

func TestTypedProbabilityStatistics(t *testing.T) {
	entropy := NewEntropy()
	for _, sample := range []float64{1, 1, 1, 1} {
		if _, err := entropy.Measure(sample); err != nil {
			t.Fatal(err)
		}
	}

	entropyOutput, _ := entropy.Measure(1)
	if entropyOutput.Value < 0.99 {
		t.Fatalf("entropy = %v, want near uniform", entropyOutput.Value)
	}

	kl := NewKLDivergence()
	for _, sample := range []PairSample{{1, 1}, {3, 1}, {4, 2}} {
		if _, err := kl.Measure(sample); err != nil {
			t.Fatal(err)
		}
	}

	klOutput, _ := kl.Measure(PairSample{2, 5})
	if !klOutput.Ready || math.IsNaN(klOutput.Value) {
		t.Fatalf("kl = %+v, want finite ready output", klOutput)
	}

	moment := NewBivariateMoment(BivariateMomentConfig{R: 1, S: 1})
	for _, sample := range []PairSample{{1, 2}, {2, 5}, {3, 7}} {
		if _, err := moment.Measure(sample); err != nil {
			t.Fatal(err)
		}
	}

	momentOutput, _ := moment.Measure(PairSample{4, 10})
	if !momentOutput.Ready || momentOutput.Value <= 0 {
		t.Fatalf("bivariate moment = %+v, want positive ready output", momentOutput)
	}
}

func TestTypedWindowAndRateStatistics(t *testing.T) {
	windows := NewWindows()
	window, err := windows.Measure([]float64{1, 2, 3, 4, 5})
	if err != nil {
		t.Fatal(err)
	}

	if window.ShortWindow <= 0 || window.LongWindow <= 0 {
		t.Fatalf("windows = %+v, want positive windows", window)
	}

	fastSlow := NewFastSlow(FastSlowConfig{FastWindow: 2})
	for _, sample := range []float64{1, 1, 4, 4} {
		if _, err := fastSlow.Measure(sample); err != nil {
			t.Fatal(err)
		}
	}

	ratio, err := fastSlow.Measure(4)
	if err != nil {
		t.Fatal(err)
	}

	if !ratio.Ready || ratio.Value <= 1 {
		t.Fatalf("fast/slow = %+v, want breakout ratio above 1", ratio)
	}
}

func TestTypedTemporalStatistics(t *testing.T) {
	priceRing := NewPriceRing()
	price, err := priceRing.Measure(101)
	if err != nil {
		t.Fatal(err)
	}

	if price.Value != 101 || !price.Ready {
		t.Fatalf("price ring = %+v, want value 101", price)
	}

	zscore := NewRollingZScore()
	for index, sample := range []float64{10, 10, 11, 12} {
		if _, err := zscore.Measure(TimedSample{
			Value: sample,
			At:    time.Unix(int64(index+1), 0),
		}); err != nil {
			t.Fatal(err)
		}
	}

	score, err := zscore.Measure(TimedSample{
		Value: 13,
		At:    time.Unix(5, 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	if !score.Ready || score.Value == 0 {
		t.Fatalf("rolling zscore = %+v, want nonzero ready score", score)
	}
}

func TestTypedMeanMedianRatio(t *testing.T) {
	ratio := NewMeanMedianRatio(MeanMedianRatioConfig{
		ShortWindow: 2,
		LongWindow:  5,
	})

	for index, sample := range []float64{10, 10, 10, 20, 20} {
		if _, err := ratio.Measure(TimedSample{
			Value: sample,
			At:    time.Unix(int64(index+1), 0),
		}); err != nil {
			t.Fatal(err)
		}
	}

	output, err := ratio.Measure(TimedSample{
		Value: 30,
		At:    time.Unix(6, 0),
	})
	if err != nil {
		t.Fatal(err)
	}

	if !output.Ready || output.Value <= 1 {
		t.Fatalf("mean/median ratio = %+v, want above 1", output)
	}
}

func BenchmarkTypedStatisticStages(b *testing.B) {
	ratio := NewMeanMedianRatio(MeanMedianRatioConfig{ShortWindow: 2, LongWindow: 8})

	b.ReportAllocs()

	for b.Loop() {
		if _, err := ratio.Measure(TimedSample{Value: 10, At: time.Now()}); err != nil {
			b.Fatal(err)
		}
	}
}
