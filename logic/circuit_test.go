package logic_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/logic"
)

func flipFlopCircuit(circuit *logic.Circuit, sample float64) float64 {
	warmupOne := datura.Acquire("test", datura.APPJSON).Poke(sample-0.5, "sample")
	_ = transport.NewFlipFlop(warmupOne, circuit)
	warmupOne.Release()

	warmupTwo := datura.Acquire("test", datura.APPJSON).Poke(sample-0.25, "sample")
	_ = transport.NewFlipFlop(warmupTwo, circuit)
	warmupTwo.Release()

	artifact := datura.Acquire("test", datura.APPJSON).Poke(sample, "sample")
	err := transport.NewFlipFlop(artifact, circuit)

	So(err, ShouldBeNil)

	return datura.Peek[float64](artifact, "output", "value")
}

func flipFlopStage(stage interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
}, sample float64) float64 {
	warmupOne := datura.Acquire("test", datura.APPJSON).Poke(sample-0.5, "sample")
	_ = transport.NewFlipFlop(warmupOne, stage)
	warmupOne.Release()

	warmupTwo := datura.Acquire("test", datura.APPJSON).Poke(sample-0.25, "sample")
	_ = transport.NewFlipFlop(warmupTwo, stage)
	warmupTwo.Release()

	artifact := datura.Acquire("test", datura.APPJSON).Poke(sample, "sample")
	err := transport.NewFlipFlop(artifact, stage)

	So(err, ShouldBeNil)

	return datura.Peek[float64](artifact, "output", "value")
}

func TestNewCircuit(testingTB *testing.T) {
	Convey("Given NewCircuit", testingTB, func() {
		circuit := logic.NewCircuit(logic.Rules{
			{
				Condition: logic.True{Operand: true},
				Then:      logic.NewConstant(1),
			},
		})

		Convey("It should return a wired circuit", func() {
			So(circuit, ShouldNotBeNil)
		})
	})
}

func TestCircuit_Observe(testingTB *testing.T) {
	Convey("Given a carried signal above its threshold", testingTB, func() {
		consequence := adaptive.NewEMA(datura.Acquire("ema-config", datura.APPJSON).Poke(2, "period"))
		alternative := adaptive.NewEMA(datura.Acquire("ema-config-alt", datura.APPJSON).Poke(2, "period"))

		circuit := logic.NewCircuit(logic.Rules{
			{
				Condition: logic.GreaterThan{
					Right: logic.NewConstant(2),
				},
				Then: consequence,
			},
			{
				Condition: logic.True{Operand: true},
				Then:      alternative,
			},
		})

		above := flipFlopCircuit(circuit, 3)

		resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
		_ = transport.NewFlipFlop(resetArtifact, circuit)

		below := flipFlopCircuit(circuit, 1)

		Convey("It should route through the matching branch", func() {
			consequenceCompare := adaptive.NewEMA(datura.Acquire("ema-config-compare", datura.APPJSON).Poke(2, "period"))
			alternativeCompare := adaptive.NewEMA(datura.Acquire("ema-config-alt-compare", datura.APPJSON).Poke(2, "period"))

			So(above, ShouldEqual, flipFlopStage(consequenceCompare, 3))
			So(below, ShouldEqual, flipFlopStage(alternativeCompare, 1))
		})
	})
}

func TestCircuit_ObserveAnd(testingTB *testing.T) {
	Convey("Given a compound And condition", testingTB, func() {
		circuit := logic.NewCircuit(logic.Rules{
			{
				Condition: logic.And{
					logic.GreaterThan{
						Right: logic.NewConstant(2),
					},
					logic.True{
						Stage: logic.NewConstant(1),
					},
				},
				Then: logic.NewConstant(10),
			},
			{
				Condition: logic.True{Operand: true},
				Then:      logic.NewConstant(20),
			},
		})

		Convey("It should require every nested condition", func() {
			So(flipFlopCircuit(circuit, 3), ShouldEqual, 10)
		})
	})
}

func TestCircuit_Reset(testingTB *testing.T) {
	Convey("Given an observed circuit", testingTB, func() {
		circuit := logic.NewCircuit(logic.Rules{
			{
				Condition: logic.True{Operand: true},
				Then:      logic.NewConstant(7),
			},
		})
		_ = flipFlopCircuit(circuit, 0)

		Convey("It should reset through the artifact", func() {
			resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
			So(transport.NewFlipFlop(resetArtifact, circuit), ShouldBeNil)
			So(flipFlopCircuit(circuit, 0), ShouldEqual, 7)
		})
	})
}

func BenchmarkCircuit_Observe(benchmark *testing.B) {
	circuit := logic.NewCircuit(logic.Rules{
		{
			Condition: logic.GreaterThan{
				Right: logic.NewConstant(2),
			},
			Then: adaptive.NewEMA(datura.Acquire("ema-config", datura.APPJSON).Poke(2, "period")),
		},
		{
			Condition: logic.True{Operand: true},
			Then:      logic.NewConstant(0),
		},
	})

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		artifact := datura.Acquire("test", datura.APPJSON).Poke(3.0, "sample")
		_ = transport.NewFlipFlop(artifact, circuit)
	}
}
