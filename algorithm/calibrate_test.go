package algorithm

import (
	"testing"

	"github.com/theapemachine/nomagique/learning"
)

func TestCalibrateMeasure(testingTB *testing.T) {
	calibrate, err := NewCalibrate(learning.RLSConfig{
		Dimension:       1,
		InitialVariance: 1000,
	})
	if err != nil {
		testingTB.Fatal(err)
	}

	var prediction float64
	lastTarget := 0.0

	for index := range 32 {
		feature := float64(index) / 32
		target := 2*feature + 1
		lastTarget = target
		output, err := calibrate.Measure(learning.RLSSample{
			Features: []float64{feature},
			Target:   target,
		})
		if err != nil {
			testingTB.Fatal(err)
		}

		prediction = output.Value
	}

	if prediction < lastTarget-0.25 || prediction > lastTarget+0.25 {
		testingTB.Fatalf(
			"prediction = %f, want within 0.25 of %f",
			prediction,
			lastTarget,
		)
	}
}

func TestCalibrateRequiresPositiveDimension(testingTB *testing.T) {
	_, err := NewCalibrate(learning.RLSConfig{
		Dimension:       0,
		InitialVariance: 1000,
	})
	if err == nil {
		testingTB.Fatal("expected invalid dimension error")
	}
}

func BenchmarkCalibrateMeasure(testingTB *testing.B) {
	calibrate, err := NewCalibrate(learning.RLSConfig{
		Dimension:       1,
		InitialVariance: 1000,
	})
	if err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := calibrate.Measure(learning.RLSSample{
			Features: []float64{0.5},
			Target:   2,
		}); err != nil {
			testingTB.Fatal(err)
		}
	}
}
