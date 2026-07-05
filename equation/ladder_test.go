package equation_test

import (
	"testing"

	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/equation"
)

func regimeLadderConfig(contagionBreak float64) equation.RegimeLadderConfig {
	return equation.RegimeLadderConfig{
		Regime: causal.RegimeConfig{
			Target:         3,
			MinHistory:     12,
			ContagionSkip:  []int{0, 3},
			ContagionBreak: contagionBreak,
		},
		Hysteresis: adaptive.HysteresisConfig{
			Window:    3,
			Threshold: 0,
		},
		Ladder: causal.LadderConfig{
			Target:          3,
			MinHistory:      12,
			TreatmentNormal: 2,
			ControlsNormal:  []int{0, 1},
			KernelBandwidth: 0.35,
		},
	}
}

func regimeLadderRows() [][]float64 {
	nodeCount := 4
	rowCount := 16
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

func TestRegimeLadderMeasure(testingTB *testing.T) {
	regimeLadder, err := equation.NewRegimeLadder(regimeLadderConfig(0.8))
	if err != nil {
		testingTB.Fatal(err)
	}

	output, err := regimeLadder.Measure(equation.RegimeLadderSample{
		Rows:      regimeLadderRows(),
		Contagion: 0,
	})
	if err != nil {
		testingTB.Fatal(err)
	}

	if output.Intervention <= 0 {
		testingTB.Fatalf("intervention = %f, want positive", output.Intervention)
	}
}

func TestRegimeLadderRequiresHysteresisWindow(testingTB *testing.T) {
	config := regimeLadderConfig(0.8)
	config.Hysteresis.Window = 0

	_, err := equation.NewRegimeLadder(config)
	if err == nil {
		testingTB.Fatal("expected hysteresis window error")
	}
}

func TestReadingNew(testingTB *testing.T) {
	reading := equation.NewReading("uplift")

	if reading == nil {
		testingTB.Fatal("expected reading")
	}
}

func BenchmarkRegimeLadderMeasure(testingTB *testing.B) {
	regimeLadder, err := equation.NewRegimeLadder(regimeLadderConfig(0.8))
	if err != nil {
		testingTB.Fatal(err)
	}

	sample := equation.RegimeLadderSample{
		Rows:      regimeLadderRows(),
		Contagion: 0,
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := regimeLadder.Measure(sample); err != nil {
			testingTB.Fatal(err)
		}
	}
}
