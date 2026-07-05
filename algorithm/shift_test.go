package algorithm

import (
	"testing"

	"github.com/theapemachine/nomagique/statistic"
)

func TestShiftMeasure(testingTB *testing.T) {
	shift := NewShift()
	var output statistic.ScalarOutput
	var err error

	for range 4 {
		output, err = shift.Measure(statistic.PairSample{
			Sample: 1,
			Paired: 1,
		})
		if err != nil {
			testingTB.Fatal(err)
		}
	}

	if output.Value != 0 {
		testingTB.Fatalf("shift = %f, want 0", output.Value)
	}

	shift = NewShift()
	pairs := []statistic.PairSample{
		{Sample: 4, Paired: 1},
		{Sample: 1, Paired: 1},
		{Sample: 1, Paired: 1},
		{Sample: 1, Paired: 4},
	}

	for _, pair := range pairs {
		output, err = shift.Measure(pair)
		if err != nil {
			testingTB.Fatal(err)
		}
	}

	if output.Value <= 0 {
		testingTB.Fatalf("shift = %f, want positive", output.Value)
	}
}

func BenchmarkShiftMeasure(testingTB *testing.B) {
	shift := NewShift()
	pairs := []statistic.PairSample{
		{Sample: 1, Paired: 2},
		{Sample: 1, Paired: 1},
		{Sample: 2, Paired: 4},
		{Sample: 1, Paired: 1},
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		for _, pair := range pairs {
			if _, err := shift.Measure(pair); err != nil {
				testingTB.Fatal(err)
			}
		}
	}
}
