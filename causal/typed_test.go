package causal

import "testing"

func causalRows(rowCount int) [][]float64 {
	rows := make([][]float64, 0, rowCount)

	for rowIndex := range rowCount {
		rows = append(rows, []float64{
			float64(rowIndex) * 0.1,
			float64(rowIndex) * 0.2,
			float64(rowIndex) * 0.5,
			float64(rowIndex) * 0.05,
		})
	}

	return rows
}

func TestBackdoorMeasure(t *testing.T) {
	backdoor := NewBackdoor(BackdoorConfig{
		Target:     3,
		Treatment:  2,
		Controls:   []int{0, 1},
		MinHistory: 12,
	})

	output, err := backdoor.Measure(BackdoorInput{
		Rows: causalRows(16),
	})
	if err != nil {
		t.Fatal(err)
	}

	if output.Effect == 0 {
		t.Fatal("expected non-zero backdoor effect")
	}
}

func TestRegimeMeasure(t *testing.T) {
	regime := NewRegime(RegimeConfig{
		Target:         3,
		MinHistory:     12,
		ContagionBreak: 0.8,
		ContagionSkip:  []int{0, 3},
	})

	output, err := regime.Measure(RegimeInput{
		Rows:      causalRows(16),
		Contagion: 0,
	})
	if err != nil {
		t.Fatal(err)
	}

	if output.RawInverted != 0 {
		t.Fatalf("raw inverted = %f, want 0", output.RawInverted)
	}

	output, err = regime.Measure(RegimeInput{
		Rows:      causalRows(16),
		Contagion: 2,
	})
	if err != nil {
		t.Fatal(err)
	}

	if output.RawInverted != 1 {
		t.Fatalf("raw inverted = %f, want 1", output.RawInverted)
	}
}

func TestLadderMeasure(t *testing.T) {
	ladder := NewLadder(LadderConfig{
		Target:          3,
		MinHistory:      12,
		TreatmentNormal: 2,
		ControlsNormal:  []int{0, 1},
		KernelBandwidth: 0.35,
	})

	output, err := ladder.Measure(LadderInput{
		Rows: causalRows(16),
	})
	if err != nil {
		t.Fatal(err)
	}

	if output.Intervention <= 0 {
		t.Fatalf("intervention = %f, want positive", output.Intervention)
	}
}

func BenchmarkLadderMeasure(testingTB *testing.B) {
	ladder := NewLadder(LadderConfig{
		Target:          3,
		MinHistory:      12,
		TreatmentNormal: 2,
		ControlsNormal:  []int{0, 1},
		KernelBandwidth: 0.35,
	})
	input := LadderInput{
		Rows: causalRows(16),
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := ladder.Measure(input); err != nil {
			testingTB.Fatal(err)
		}
	}
}
