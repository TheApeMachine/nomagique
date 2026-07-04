package logic_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/logic"
)

func TestIntegration(t *testing.T) {
	Convey("Given logic stages composed through nomagique.Number", t, func() {
		Convey("When Constant emits a fixed scalar", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			err := nomagique.RoundTripArtifact(artifact, constantStage(42))

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 42)
		})

		Convey("When Circuit routes above threshold to consequence branch", func() {
			circuit := logic.NewCircuit(circuitConfig(), logic.Rules{
				{
					Condition: logic.GreaterThan{Right: constantStage(2)},
					Then:      constantStage(7),
				},
				{
					Condition: logic.True{Operand: true},
					Then:      constantStage(0),
				},
			})

			artifact := scalarWire(datura.Acquire("test", datura.APPJSON), 3)
			_ = nomagique.RoundTripArtifact(artifact, nomagique.Number(circuit))
			artifact = scalarWire(datura.Acquire("test", datura.APPJSON), 4)
			err := nomagique.RoundTripArtifact(artifact, nomagique.Number(circuit))

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 7)
		})

		Convey("When Circuit falls through to default branch", func() {
			circuit := logic.NewCircuit(circuitConfig(), logic.Rules{
				{
					Condition: logic.GreaterThan{Right: constantStage(10)},
					Then:      constantStage(1),
				},
				{
					Condition: logic.True{Operand: true},
					Then:      constantStage(99),
				},
			})

			artifact := scalarWire(datura.Acquire("test", datura.APPJSON), 1)
			err := nomagique.RoundTripArtifact(artifact, nomagique.Number(circuit))

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 99)
		})
	})
}
