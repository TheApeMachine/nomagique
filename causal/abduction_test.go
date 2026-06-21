package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func abductionConfig(linear bool, treatment int, intervention float64) *datura.Artifact {
	config := datura.Acquire("abduction-config", datura.APPJSON).
		Poke(float64(3), "config", "target").
		Poke(float64(treatment), "config", "treatment").
		Poke(intervention, "config", "intervention").
		Poke(float64(12), "config", "minHistory").
		Poke([]float64{0, 1, 2}, "config", "features")

	if linear {
		config.Poke(float64(1), "config", "linear")
	}

	return config
}

func abductionTable(rowCount int, linear bool) *datura.Artifact {
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

	return datura.Acquire("abduction-inbound", datura.APPJSON).
		Poke(float64(rowCount), "table", "rowCount").
		Poke(float64(nodeCount), "table", "nodeCount").
		Poke(flat, "table", "rows")
}

func lastRowTarget(config *datura.Artifact, table *datura.Artifact) float64 {
	flat := datura.Peek[[]float64](table, "table", "rows")
	nodeCount := int(datura.Peek[float64](table, "table", "nodeCount"))

	return flat[len(flat)-nodeCount+int(datura.Peek[float64](config, "config", "target"))]
}

func lastRowTreatment(config *datura.Artifact, table *datura.Artifact) float64 {
	flat := datura.Peek[[]float64](table, "table", "rows")
	nodeCount := int(datura.Peek[float64](table, "table", "nodeCount"))
	treatment := int(datura.Peek[float64](config, "config", "treatment"))

	return flat[len(flat)-nodeCount+treatment]
}

func TestAbduction_Read_Linear(testingTB *testing.T) {
	Convey("Given a fitted linear structural model through the pipeline", testingTB, func() {
		config := abductionConfig(true, 2, 20)
		table := abductionTable(16, true)
		stage := NewAbduction(config)
		err := transport.NewFlipFlop(table, stage)

		So(err, ShouldBeNil)

		Convey("It should preserve the abducted level at the observed treatment", func() {
			restoreConfig := abductionConfig(true, 2, lastRowTreatment(config, table))
			restoreTable := abductionTable(16, true)
			restoreStage := NewAbduction(restoreConfig)
			err := transport.NewFlipFlop(restoreTable, restoreStage)

			So(err, ShouldBeNil)
			So(
				datura.Peek[float64](restoreTable, "output", "counterfactual"),
				ShouldAlmostEqual,
				lastRowTarget(restoreConfig, restoreTable),
				1e-9,
			)
		})

		Convey("It should move the counterfactual when treatment changes", func() {
			movedConfig := abductionConfig(true, 2, 20)
			movedTable := abductionTable(16, true)
			movedStage := NewAbduction(movedConfig)
			err := transport.NewFlipFlop(movedTable, movedStage)

			So(err, ShouldBeNil)
			So(
				datura.Peek[float64](movedTable, "output", "counterfactual"),
				ShouldNotAlmostEqual,
				lastRowTarget(movedConfig, movedTable),
				1e-9,
			)
		})
	})
}

func TestAbduction_Read_NonLinear(testingTB *testing.T) {
	Convey("Given a fitted nonlinear model through the pipeline", testingTB, func() {
		config := abductionConfig(false, 2, 2.0)
		table := abductionTable(16, false)
		stage := NewAbduction(config)
		err := transport.NewFlipFlop(table, stage)

		So(err, ShouldBeNil)

		observedTarget := lastRowTarget(config, table)
		uplift := datura.Peek[float64](table, "output", "uplift")
		counterfactual := datura.Peek[float64](table, "output", "counterfactual")
		noise := datura.Peek[float64](table, "output", "noise")

		Convey("It should return a finite abductive counterfactual read", func() {
			So(noise, ShouldNotEqual, 0)
			So(counterfactual, ShouldAlmostEqual, observedTarget+uplift, 1e-9)
		})
	})
}

func BenchmarkAbduction_Read(testingTB *testing.B) {
	config := abductionConfig(false, 2, 2.0)
	table := abductionTable(16, false)
	stage := NewAbduction(config)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(table, stage)
	}
}
