package algorithm_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/algorithm"
)

func backdoorConfig() *datura.Artifact {
	return datura.Acquire("backdoor-config", datura.APPJSON).
		Poke(float64(3), "target").
		Poke(float64(2), "treatment").
		Poke([]float64{0, 1}, "controls").
		Poke(float64(12), "minHistory")
}

func backdoorTable(rowCount int) *datura.Artifact {
	nodeCount := 4
	flat := make([]float64, 0, rowCount*nodeCount)

	for rowIndex := range rowCount {
		flat = append(flat,
			float64(rowIndex)*0.1,
			float64(rowIndex)*0.2,
			float64(rowIndex)*0.5,
			float64(rowIndex)*0.05,
		)
	}

	return datura.Acquire("backdoor-table", datura.APPJSON).
		Poke(float64(rowCount), "table", "rowCount").
		Poke(float64(nodeCount), "table", "nodeCount").
		Poke(flat, "table", "rows")
}

func TestBackdoorRead(testingTB *testing.T) {
	Convey("Given aligned node streams with causal structure", testingTB, func() {
		backdoor := algorithm.NewBackdoor(backdoorConfig())
		artifact := backdoorTable(16)
		err := nomagique.RoundTripArtifact(artifact, backdoor)

		So(err, ShouldBeNil)

		Convey("It should return a finite backdoor effect", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldNotEqual, 0)
		})
	})
}

func BenchmarkBackdoorRead(testingTB *testing.B) {
	backdoor := algorithm.NewBackdoor(backdoorConfig())
	artifact := backdoorTable(16)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = nomagique.RoundTripArtifact(artifact, backdoor)
	}
}
