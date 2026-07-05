package vector

import (
	"math"
	"testing"
)

func mapperVector() FeatureVector {
	return FeatureVector{
		Features: []float64{100.9, 101.1, 2500},
		Inputs:   []string{"bid", "ask", "volume"},
	}
}

func TestMapperMeasure(t *testing.T) {
	mapper := NewMapper(MapperConfig{
		Mappings: []Mapping{
			{
				OutputKey: "spreadPoints",
				Inputs:    []string{"ask", "bid"},
				Inverts:   []string{"bid"},
				Op:        "sum",
			},
			{
				OutputKey: "meanPrice",
				Inputs:    []string{"bid", "ask"},
				Op:        "mean",
			},
		},
	})

	output, err := mapper.Measure(mapperVector())
	if err != nil {
		t.Fatal(err)
	}

	spread, exists := output.Value("spreadPoints")
	if !exists {
		t.Fatal("spreadPoints missing")
	}

	if math.Abs(spread-0.2) > 1e-12 {
		t.Fatalf("spreadPoints = %.12f, want 0.2", spread)
	}

	mean, exists := output.Value("meanPrice")
	if !exists || mean != 101 {
		t.Fatalf("meanPrice = %f/%t, want 101/true", mean, exists)
	}
}

func TestMapperRejectsDivideByZero(t *testing.T) {
	mapper := NewMapper(MapperConfig{
		Mappings: []Mapping{
			{
				OutputKey: "ratio",
				Inputs:    []string{"volume", "zero"},
				Op:        "ratio",
			},
		},
	})

	_, err := mapper.Measure(FeatureVector{
		Features: []float64{2500, 0},
		Inputs:   []string{"volume", "zero"},
	})
	if err == nil {
		t.Fatal("expected divide by zero error")
	}
}

func BenchmarkMapperMeasure(testingTB *testing.B) {
	mapper := NewMapper(MapperConfig{
		Mappings: []Mapping{
			{
				OutputKey: "spreadPoints",
				Inputs:    []string{"ask", "bid"},
				Inverts:   []string{"bid"},
				Op:        "sum",
			},
		},
	})
	vector := mapperVector()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := mapper.Measure(vector); err != nil {
			testingTB.Fatal(err)
		}
	}
}
