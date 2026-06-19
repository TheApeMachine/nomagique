package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func abductionArtifact(
	rowCount int,
	linear bool,
	treatment int,
	intervention float64,
) *datura.Artifact {
	nodeCount := 4
	flat := make([]float64, 0, rowCount*nodeCount)

	for rowIndex := range rowCount {
		if linear {
			flat = append(flat,
				float64(rowIndex),
				float64(rowIndex)*0.5,
				float64(rowIndex)*2,
				float64(rowIndex)*0.25,
			)

			continue
		}

		flat = append(flat,
			float64(rowIndex)*0.1,
			float64(rowIndex)*0.2,
			float64(rowIndex)*0.3,
			float64(rowIndex)*0.05,
		)
	}

	artifact := datura.Acquire("test", datura.APPJSON).
		Poke(float64(3), "config", "target").
		Poke(float64(treatment), "config", "treatment").
		Poke(intervention, "config", "intervention").
		Poke(float64(12), "config", "minHistory").
		Poke([]float64{0, 1, 2}, "config", "features").
		Poke(float64(rowCount), "table", "rowCount").
		Poke(float64(nodeCount), "table", "nodeCount").
		Poke(flat, "table", "rows")

	if linear {
		artifact.Poke(float64(1), "config", "linear")
	}

	return artifact
}

func lastRowTarget(artifact *datura.Artifact) float64 {
	flat := datura.Peek[[]float64](artifact, "table", "rows")
	nodeCount := int(datura.Peek[float64](artifact, "table", "nodeCount"))

	return flat[len(flat)-nodeCount+int(datura.Peek[float64](artifact, "config", "target"))]
}

func lastRowTreatment(artifact *datura.Artifact) float64 {
	flat := datura.Peek[[]float64](artifact, "table", "rows")
	nodeCount := int(datura.Peek[float64](artifact, "table", "nodeCount"))
	treatment := int(datura.Peek[float64](artifact, "config", "treatment"))

	return flat[len(flat)-nodeCount+treatment]
}

func TestAbduction_Read_Linear(testingTB *testing.T) {
	Convey("Given a fitted linear structural model through the pipeline", testingTB, func() {
		stage := NewAbduction()
		artifact := abductionArtifact(16, true, 2, 20)
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should preserve the abducted level at the observed treatment", func() {
			restoreStage := NewAbduction()
			restoreArtifact := abductionArtifact(16, true, 2, lastRowTreatment(artifact))
			err := transport.NewFlipFlop(restoreArtifact, restoreStage)

			So(err, ShouldBeNil)
			So(
				datura.Peek[float64](restoreArtifact, "output", "counterfactual"),
				ShouldAlmostEqual,
				lastRowTarget(restoreArtifact),
				1e-9,
			)
		})

		Convey("It should move the counterfactual when treatment changes", func() {
			movedStage := NewAbduction()
			movedArtifact := abductionArtifact(16, true, 2, 20)
			err := transport.NewFlipFlop(movedArtifact, movedStage)

			So(err, ShouldBeNil)
			So(
				datura.Peek[float64](movedArtifact, "output", "counterfactual"),
				ShouldNotAlmostEqual,
				lastRowTarget(movedArtifact),
				1e-9,
			)
		})
	})
}

func TestAbduction_Read_NonLinear(testingTB *testing.T) {
	Convey("Given a fitted nonlinear model through the pipeline", testingTB, func() {
		stage := NewAbduction()
		artifact := abductionArtifact(16, false, 2, 2.0)
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		observedTarget := lastRowTarget(artifact)
		uplift := datura.Peek[float64](artifact, "output", "uplift")
		counterfactual := datura.Peek[float64](artifact, "output", "counterfactual")
		noise := datura.Peek[float64](artifact, "output", "noise")

		Convey("It should return a finite abductive counterfactual read", func() {
			So(noise, ShouldNotEqual, 0)
			So(counterfactual, ShouldAlmostEqual, observedTarget+uplift, 1e-9)
		})
	})
}

func BenchmarkAbduction_Read(testingTB *testing.B) {
	stage := NewAbduction()
	artifact := abductionArtifact(16, false, 2, 2.0)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
