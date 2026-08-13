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
			ContagionSkip:  []int{0, 3},
			ContagionBreak: contagionBreak,
		},
		Hysteresis: adaptive.HysteresisConfig{
			Window:    3,
			Threshold: 0,
		},
		Ladder: causal.LadderConfig{
			Target:          3,
			TreatmentNormal: 2,
			ControlsNormal:  []int{0, 1},
		},
	}
}

func regimeLadderRows() [][]float64 {
	nodeCount := 4
	rowCount := 16
	rows := make([][]float64, 0, rowCount)

	for rowIndex := range rowCount {
		controlPrimary := float64(rowIndex % 3)
		controlSecondary := float64((rowIndex*rowIndex + rowIndex) % 5)
		treatment := float64(rowIndex)*0.5 + controlPrimary*0.2 - controlSecondary*0.1
		target := controlPrimary*0.3 - controlSecondary*0.2 + treatment*0.8
		row := make([]float64, 0, nodeCount)
		row = append(row,
			controlPrimary,
			controlSecondary,
			treatment,
			target,
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

	value, err := reading.Measure(map[string]float64{"uplift": 0.42})
	if err != nil {
		testingTB.Fatal(err)
	}

	if value != 0.42 {
		testingTB.Fatalf("value = %f, want 0.42", value)
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
