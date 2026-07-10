package adaptive

import (
	"math"
	"testing"
	"time"
)

func TestTypedAdaptiveStages(t *testing.T) {
	accumulator := NewAccumulator()
	accumulated, err := accumulator.Measure(1.5)
	if err != nil {
		t.Fatal(err)
	}

	if accumulated.Value != 1.5 || !accumulated.Ready {
		t.Fatalf("accumulator = %+v, want ready value 1.5", accumulated)
	}

	compression := NewCompression()
	if _, err := compression.Measure(10); err != nil {
		t.Fatal(err)
	}

	compressed, err := compression.Measure(8)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(compressed.Value-0.2) > 1e-9 {
		t.Fatalf("compression = %v, want 0.2", compressed.Value)
	}

	positiveOnly := NewPositiveOnly(true)
	positive, err := positiveOnly.Measure(-3)
	if err != nil {
		t.Fatal(err)
	}

	if positive.Value != 0 {
		t.Fatalf("positive-only = %v, want 0", positive.Value)
	}
}

func TestTypedAdaptiveHistoryStages(t *testing.T) {
	delta := NewDelta()
	_, _ = delta.Measure(10)
	deltaOutput, err := delta.Measure(14)
	if err != nil {
		t.Fatal(err)
	}

	if deltaOutput.Value != 1 || !deltaOutput.Ready {
		t.Fatalf("delta = %+v, want ready value 1", deltaOutput)
	}

	momentum := NewMomentum()
	_, _ = momentum.Measure(10)
	momentumOutput, err := momentum.Measure(14)
	if err != nil {
		t.Fatal(err)
	}

	if momentumOutput.Value != 1 || !momentumOutput.Ready {
		t.Fatalf("momentum = %+v, want ready value 1", momentumOutput)
	}

	extent := NewRange()
	_, _ = extent.Measure(10)
	rangeOutput, err := extent.Measure(14)
	if err != nil {
		t.Fatal(err)
	}

	if rangeOutput.Value != 4 || !rangeOutput.Ready {
		t.Fatalf("range = %+v, want ready value 4", rangeOutput)
	}
}

func TestTypedAdaptiveDistributionStages(t *testing.T) {
	variance := NewVariance()
	for _, sample := range []float64{10, 22, 30, 40} {
		if _, err := variance.Measure(sample); err != nil {
			t.Fatal(err)
		}
	}

	varianceOutput, err := variance.Measure(44)
	if err != nil {
		t.Fatal(err)
	}

	if varianceOutput.Value <= 0 || !varianceOutput.Ready {
		t.Fatalf("variance = %+v, want positive ready variance", varianceOutput)
	}

	zscore := NewZScore()
	for _, sample := range []float64{10, 22, 30, 40} {
		if _, err := zscore.Measure(sample); err != nil {
			t.Fatal(err)
		}
	}

	zscoreOutput, err := zscore.Measure(44)
	if err != nil {
		t.Fatal(err)
	}

	if zscoreOutput.Value == 0 || !zscoreOutput.Ready {
		t.Fatalf("zscore = %+v, want nonzero ready score", zscoreOutput)
	}
}

func TestTypedAdaptiveTemporalStages(t *testing.T) {
	logReturn := NewLogReturn(LogReturnConfig{ReturnLag: 1, LongWindow: 3})
	first, err := logReturn.Measure(LogReturnSample{Value: 100, At: time.Unix(1, 0)})
	if err != nil {
		t.Fatal(err)
	}

	if !first.Ready || first.Value != 0 {
		t.Fatalf("first log return = %+v, want ready zero return", first)
	}

	second, err := logReturn.Measure(LogReturnSample{Value: 110, At: time.Unix(2, 0)})
	if err != nil {
		t.Fatal(err)
	}

	if !second.Ready || second.Value <= 0 {
		t.Fatalf("log return = %+v, want positive ready return", second)
	}

	timeElastic := NewTimeElastic(TimeElasticConfig{
		Halflife: time.Second,
		Epsilon:  0.01,
	})
	_, _ = timeElastic.Measure(TimedValue{Value: 10, At: time.Unix(1, 0)})

	elastic, err := timeElastic.Measure(TimedValue{Value: 12, At: time.Unix(2, 0)})
	if err != nil {
		t.Fatal(err)
	}

	if !elastic.Ready || elastic.Value <= 1 {
		t.Fatalf("time elastic = %+v, want ready ratio above 1", elastic)
	}
}

func TestTypedFracDiffAndHysteresis(t *testing.T) {
	fractional := NewFracDiff()
	_, _ = fractional.Measure(10)
	fracOutput, err := fractional.Measure(11)
	if err != nil {
		t.Fatal(err)
	}

	if !fracOutput.Ready || fracOutput.Value == 0 {
		t.Fatalf("fracdiff = %+v, want nonzero ready output", fracOutput)
	}

	value, ready, err := FractionalDifferenceValue([]float64{0, 0.25, 0.5, 1})
	if err != nil {
		t.Fatal(err)
	}

	if !ready || value == 0 {
		t.Fatalf("fractional difference = %v ready %v, want nonzero ready", value, ready)
	}

	hysteresis := NewHysteresis(HysteresisConfig{Window: 2, Threshold: 0.5})
	_, _ = hysteresis.Measure(0.6)
	gated, err := hysteresis.Measure(0.7)
	if err != nil {
		t.Fatal(err)
	}

	if gated.Value != 1 {
		t.Fatalf("hysteresis = %+v, want value 1", gated)
	}
}

func BenchmarkTypedAdaptiveStages(b *testing.B) {
	zscore := NewZScore()

	b.ReportAllocs()

	for b.Loop() {
		if _, err := zscore.Measure(40); err != nil {
			b.Fatal(err)
		}
	}
}
