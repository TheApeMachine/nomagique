package logic_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/logic"
)

func TestIntegration(t *testing.T) {
	Convey("Given logic stages composed through nomagique.Number", t, func() {
		Convey("When Constant emits a fixed scalar", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			err := transport.NewFlipFlop(artifact, logic.NewConstant(42))

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 42)
		})

		Convey("When Circuit routes above threshold to consequence branch", func() {
			consequence := adaptive.NewEMA(datura.Acquire("ema-config", datura.APPJSON).
				Poke("sample", "input").
				Poke(2, "period").
				Poke(2, "smoothing"))
			circuit := logic.NewCircuit(logic.Rules{
				{
					Condition: logic.GreaterThan{Right: logic.NewConstant(2)},
					Then:      consequence,
				},
				{
					Condition: logic.True{Operand: true},
					Then:      logic.NewConstant(0),
				},
			})

			artifact := scalarWire(datura.Acquire("test", datura.APPJSON), 3)
			_ = transport.NewFlipFlop(artifact, nomagique.Number(circuit))
			artifact = scalarWire(datura.Acquire("test", datura.APPJSON), 4)
			err := transport.NewFlipFlop(artifact, nomagique.Number(circuit))

			So(err, ShouldBeNil)

			expectedArtifact := scalarWire(datura.Acquire("test", datura.APPJSON), 3)
			reference := adaptive.NewEMA(datura.Acquire("ema-config-reference", datura.APPJSON).
				Poke("sample", "input").
				Poke(2, "period").
				Poke(2, "smoothing"))
			_ = transport.NewFlipFlop(expectedArtifact, reference)
			expectedArtifact = scalarWire(datura.Acquire("test", datura.APPJSON), 4)
			err = transport.NewFlipFlop(expectedArtifact, reference)

			So(err, ShouldBeNil)
			So(
				datura.Peek[float64](artifact, "output", "value"),
				ShouldEqual,
				datura.Peek[float64](expectedArtifact, "output", "value"),
			)
		})

		Convey("When Circuit falls through to default branch", func() {
			circuit := logic.NewCircuit(logic.Rules{
				{
					Condition: logic.GreaterThan{Right: logic.NewConstant(10)},
					Then:      logic.NewConstant(1),
				},
				{
					Condition: logic.True{Operand: true},
					Then:      logic.NewConstant(99),
				},
			})

			artifact := scalarWire(datura.Acquire("test", datura.APPJSON), 1)
			err := transport.NewFlipFlop(artifact, nomagique.Number(circuit))

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 99)
		})
	})
}
