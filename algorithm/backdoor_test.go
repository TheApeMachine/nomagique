package algorithm_test

import (
	"testing"

	"github.com/theapemachine/nomagique/algorithm"
	"github.com/theapemachine/nomagique/causal"
)

func backdoorConfig() causal.BackdoorConfig {
	return causal.BackdoorConfig{
		Target:     3,
		Treatment:  2,
		Controls:   []int{0, 1},
		MinHistory: 12,
	}
}

func backdoorRows(rowCount int) [][]float64 {
	nodeCount := 4
	rows := make([][]float64, 0, rowCount)

	for rowIndex := range rowCount {
		row := make([]float64, 0, nodeCount)
		row = append(row,
			float64(rowIndex)*0.1,
			float64(rowIndex)*0.2,
			float64(rowIndex)*0.5,
			float64(rowIndex)*0.05,
		)
		rows = append(rows, row)
	}

	return rows
}

func TestBackdoorMeasure(testingTB *testing.T) {
	backdoor := algorithm.NewBackdoor(backdoorConfig())
	output, err := backdoor.Measure(causal.BackdoorInput{
		Rows: backdoorRows(16),
	})
	if err != nil {
		testingTB.Fatal(err)
	}

	if output.Value == 0 {
		testingTB.Fatal("expected finite non-zero backdoor effect")
	}
}

func BenchmarkBackdoorMeasure(testingTB *testing.B) {
	backdoor := algorithm.NewBackdoor(backdoorConfig())
	input := causal.BackdoorInput{
		Rows: backdoorRows(16),
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := backdoor.Measure(input); err != nil {
			testingTB.Fatal(err)
		}
	}
}
