package algorithm

import "testing"

func TestMoveBaselineMeasure(testingTB *testing.T) {
	baseline := NewMoveBaseline(MoveBaselineConfig{
		MinObs:  anchorMoveMinObs,
		PathCap: 256,
	})

	for index := range anchorMoveMinObs {
		output, err := baseline.Measure(0.0001 + float64(index%2)*0.00005)
		if err != nil {
			testingTB.Fatal(err)
		}

		if output.Ready != 0 {
			testingTB.Fatalf("warmup ready = %f, want 0", output.Ready)
		}
	}

	output, err := baseline.Measure(0.00001)
	if err != nil {
		testingTB.Fatal(err)
	}

	if output.Ready != 1 {
		testingTB.Fatalf("ready = %f, want 1", output.Ready)
	}

	if output.Moved != 0 {
		testingTB.Fatalf("moved = %f, want 0", output.Moved)
	}

	if output.StallMargin <= 0 || output.StallMargin > 1 {
		testingTB.Fatalf("stall margin = %f, want in (0,1]", output.StallMargin)
	}
}

const (
	anchorMoveMinObs = 12
)
