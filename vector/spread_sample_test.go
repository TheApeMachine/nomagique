package vector

import (
	"math"
	"testing"
)

func TestSpreadSampleMeasure(t *testing.T) {
	stage := NewSpreadSample(SpreadSampleConfig{
		Inputs:    []string{"bid", "ask"},
		OutputKey: "spread",
	})

	output, err := stage.Measure(FeatureVector{
		Features: []float64{100.9, 101.1},
		Inputs:   []string{"bid", "ask"},
	})
	if err != nil {
		t.Fatal(err)
	}

	want := (101.1 - 100.9) / ((101.1 + 100.9) / 2)
	if math.Abs(output.Value.Value-want) > 1e-12 {
		t.Fatalf("spread = %.12f, want %.12f", output.Value.Value, want)
	}

	if value, exists := output.Vector.Value("spread"); !exists || value != output.Value.Value {
		t.Fatalf("vector spread = %f/%t", value, exists)
	}
}

func TestSpreadSampleRejectsMissingInput(t *testing.T) {
	stage := NewSpreadSample(SpreadSampleConfig{
		Inputs:    []string{"bid", "ask"},
		OutputKey: "spread",
	})

	_, err := stage.Measure(FeatureVector{
		Features: []float64{100.9},
		Inputs:   []string{"bid"},
	})
	if err == nil {
		t.Fatal("expected missing input error")
	}
}
