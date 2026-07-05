package vector

import (
	"math"
	"testing"
)

func tickerExtractorConfig() FeatureExtractorConfig {
	return FeatureExtractorConfig{
		Channels: map[string]FeatureScopeConfig{
			"ticker": {
				Root:         "data",
				ElementIndex: 0,
				Inputs: []string{
					"volume", "vwap", "last", "bid", "ask", "change_pct",
				},
			},
		},
	}
}

func tickerInput() FeatureInput {
	return FeatureInput{
		Channel: "ticker",
		Rows: []FeatureRow{
			NewFeatureRow(
				NamedValue{Name: "volume", Value: 2500},
				NamedValue{Name: "vwap", Value: 100},
				NamedValue{Name: "last", Value: 101},
				NamedValue{Name: "bid", Value: 100.9},
				NamedValue{Name: "ask", Value: 101.1},
				NamedValue{Name: "change_pct", Value: 1},
			),
		},
	}
}

func TestFeatureExtractorMeasure(t *testing.T) {
	extractor := NewFeatureExtractor(tickerExtractorConfig())

	output, err := extractor.Measure(tickerInput())
	if err != nil {
		t.Fatal(err)
	}

	wantInputs := []string{"volume", "vwap", "last", "bid", "ask", "change_pct"}
	for index, input := range wantInputs {
		if output.Inputs[index] != input {
			t.Fatalf("input %d = %s, want %s", index, output.Inputs[index], input)
		}
	}

	wantFeatures := []float64{2500, 100, 101, 100.9, 101.1, 1}
	for index, want := range wantFeatures {
		if output.Features[index] != want {
			t.Fatalf("feature %d = %f, want %f", index, output.Features[index], want)
		}
	}

	if output.SourceRoot != "data" {
		t.Fatalf("source root = %s, want data", output.SourceRoot)
	}
}

func TestFeatureExtractorMeasureRowRoot(t *testing.T) {
	extractor := NewFeatureExtractor(FeatureExtractorConfig{
		FeatureScopeConfig: FeatureScopeConfig{
			Root:   ".",
			Inputs: []string{"bid", "ask", "last"},
		},
	})

	output, err := extractor.Measure(FeatureInput{
		Row: NewFeatureRow(
			NamedValue{Name: "bid", Value: 100.9},
			NamedValue{Name: "ask", Value: 101.1},
			NamedValue{Name: "last", Value: 101},
		),
	})
	if err != nil {
		t.Fatal(err)
	}

	wantFeatures := []float64{100.9, 101.1, 101}
	for index, want := range wantFeatures {
		if output.Features[index] != want {
			t.Fatalf("feature %d = %f, want %f", index, output.Features[index], want)
		}
	}

	if output.SourceRoot != "." {
		t.Fatalf("source root = %s, want .", output.SourceRoot)
	}
}

func TestFeatureExtractorRejectsNonFinite(t *testing.T) {
	extractor := NewFeatureExtractor(FeatureExtractorConfig{
		FeatureScopeConfig: FeatureScopeConfig{
			Root:   ".",
			Inputs: []string{"volume"},
		},
	})

	_, err := extractor.Measure(FeatureInput{
		Row: NewFeatureRow(NamedValue{Name: "volume", Value: math.NaN()}),
	})
	if err == nil {
		t.Fatal("expected non-finite sample error")
	}
}

func BenchmarkFeatureExtractorMeasure(testingTB *testing.B) {
	extractor := NewFeatureExtractor(tickerExtractorConfig())
	input := tickerInput()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := extractor.Measure(input); err != nil {
			testingTB.Fatal(err)
		}
	}
}
