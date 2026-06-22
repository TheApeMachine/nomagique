package causal

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func doConfig() *datura.Artifact {
	return datura.Acquire("do-config", datura.APPJSON).
		Poke(float64(3), "target").
		Poke(float64(2), "treatment").
		Poke(20.0, "level").
		Poke([]float64{0, 1}, "controls").
		Poke(float64(12), "minHistory")
}

func doTableInbound() *datura.Artifact {
	rows := make([][]float64, 16)
	nodeCount := 4
	flat := make([]float64, 0, len(rows)*nodeCount)

	for index := range rows {
		rows[index] = []float64{
			float64(index),
			float64(index) * 0.5,
			float64(index) * 2,
			float64(index) * 0.25,
		}
		flat = append(flat, rows[index]...)
	}

	return datura.Acquire("do-inbound", datura.APPJSON).
		Poke(float64(len(rows)), "table", "rowCount").
		Poke(float64(nodeCount), "table", "nodeCount").
		Poke(flat, "table", "rows")
}

func TestDo_Read(testingTB *testing.T) {
	Convey("Given a linear causal table", testingTB, func() {
		stage := NewDo(doConfig())
		artifact := doTableInbound()
		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		expectation := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return a finite interventional expectation", func() {
			So(expectation, ShouldNotEqual, 0)
		})
	})
}

func BenchmarkDo_Read(testingTB *testing.B) {
	stage := NewDo(doConfig())
	artifact := doTableInbound()

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
