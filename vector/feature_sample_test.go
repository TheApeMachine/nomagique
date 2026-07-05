package vector

import "testing"

func TestFeatureSampleMeasure(t *testing.T) {
	stage := NewFeatureSample(FeatureSampleConfig{
		FeatureIndex: 1,
		OutputKey:    "sample",
	})

	output, err := stage.Measure(FeatureVector{
		Features: []float64{10, 20, 30},
		Inputs:   []string{"a", "b", "c"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if output.Value.Value != 20 {
		t.Fatalf("sample = %f, want 20", output.Value.Value)
	}

	if value, exists := output.Vector.Value("sample"); !exists || value != 20 {
		t.Fatalf("vector sample = %f/%t, want 20/true", value, exists)
	}
}

func TestFeatureSampleRejectsOutOfRange(t *testing.T) {
	stage := NewFeatureSample(FeatureSampleConfig{
		FeatureIndex: 3,
		OutputKey:    "sample",
	})

	_, err := stage.Measure(FeatureVector{
		Features: []float64{10},
		Inputs:   []string{"a"},
	})
	if err == nil {
		t.Fatal("expected out-of-range feature error")
	}
}

func BenchmarkFeatureSampleMeasure(testingTB *testing.B) {
	stage := NewFeatureSample(FeatureSampleConfig{
		FeatureIndex: 1,
		OutputKey:    "sample",
	})
	vector := FeatureVector{
		Features: []float64{10, 20, 30},
		Inputs:   []string{"a", "b", "c"},
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := stage.Measure(vector); err != nil {
			testingTB.Fatal(err)
		}
	}
}
