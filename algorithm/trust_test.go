package algorithm

import (
	"testing"

	"github.com/theapemachine/nomagique/learning"
)

func TestTrustMeasure(testingTB *testing.T) {
	trust := NewTrust()
	var score TrustOutput
	var err error

	for step := range 16 {
		predicted := float64(step + 10)
		score, err = trust.Measure(learning.LearningPair{
			Predicted: predicted,
			Actual:    predicted,
		})
		if err != nil {
			testingTB.Fatal(err)
		}
	}

	score, err = trust.Measure(learning.LearningPair{
		Predicted: 26,
		Actual:    26,
	})
	if err != nil {
		testingTB.Fatal(err)
	}

	if score.Value <= 0 {
		testingTB.Fatalf("trust score = %f, want positive", score.Value)
	}
}

func BenchmarkTrustMeasure(testingTB *testing.B) {
	trust := NewTrust()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		step := float64(testingTB.N % 16)
		if _, err := trust.Measure(learning.LearningPair{
			Predicted: step + 1,
			Actual:    (step + 1) * 2,
		}); err != nil {
			testingTB.Fatal(err)
		}
	}
}
