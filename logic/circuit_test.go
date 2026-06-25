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
	artifact := scalarWire(datura.Acquire("test", datura.APPJSON), sample)
	err := transport.NewFlipFlop(artifact, circuit)

	So(err, ShouldBeNil)

	return datura.Peek[float64](artifact, "output", "value")
}

func TestNewCircuit(testingTB *testing.T) {
	Convey("Given NewCircuit", testingTB, func() {
		circuit := logic.NewCircuit(circuitConfig(), logic.Rules{
			{
				Condition: logic.True{Operand: true},
				Then:      constantStage(1),
			},
		})

		Convey("It should return a wired circuit", func() {
			So(circuit, ShouldNotBeNil)
		})
	})
}

func TestCircuitRead(testingTB *testing.T) {
	Convey("Given a carried signal above its threshold", testingTB, func() {
		circuit := logic.NewCircuit(circuitConfig(), logic.Rules{
			{
				Condition: logic.GreaterThan{
					Right: constantStage(2),
				},
				Then: constantStage(10),
			},
			{
				Condition: logic.True{Operand: true},
				Then:      constantStage(20),
			},
		})

		above := flipFlopCircuit(circuit, 3)

		resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
		_ = transport.NewFlipFlop(resetArtifact, circuit)

		below := flipFlopCircuit(circuit, 1)

		Convey("It should route through the matching branch", func() {
			So(above, ShouldEqual, 10)
			So(below, ShouldEqual, 20)
		})
	})
}

func TestCircuitReadAnd(testingTB *testing.T) {
	Convey("Given a compound And condition", testingTB, func() {
		circuit := logic.NewCircuit(circuitConfig(), logic.Rules{
			{
				Condition: logic.And{
					logic.GreaterThan{
						Right: constantStage(2),
					},
					logic.True{
						Stage: constantStage(1),
					},
				},
				Then: constantStage(10),
			},
			{
				Condition: logic.True{Operand: true},
				Then:      constantStage(20),
			},
		})

		Convey("It should require every nested condition", func() {
			So(flipFlopCircuit(circuit, 3), ShouldEqual, 10)
		})
	})
}

func TestCircuit_Reset(testingTB *testing.T) {
	Convey("Given an observed circuit", testingTB, func() {
		circuit := logic.NewCircuit(circuitConfig(), logic.Rules{
			{
				Condition: logic.True{Operand: true},
				Then:      constantStage(7),
			},
		})
		_ = flipFlopCircuit(circuit, 0)

		Convey("It should reset through the artifact", func() {
			resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
			packed := resetArtifact.Pack()
			resetArtifact.Release()

			So(len(packed), ShouldBeGreaterThan, 0)

			_, writeErr := circuit.Write(packed)
			So(writeErr, ShouldBeNil)
			So(flipFlopCircuit(circuit, 0), ShouldEqual, 7)
		})
	})
}

func BenchmarkCircuitRead(benchmark *testing.B) {
	circuit := logic.NewCircuit(circuitConfig(), logic.Rules{
		{
			Condition: logic.GreaterThan{
				Right: constantStage(2),
			},
			Then: adaptive.NewEMA(datura.Acquire("ema-config", datura.APPJSON).Poke(2, "period")),
		},
		{
			Condition: logic.True{Operand: true},
			Then:      constantStage(0),
		},
	})

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		artifact := scalarWire(datura.Acquire("test", datura.APPJSON), 3.0)
		_ = transport.NewFlipFlop(artifact, circuit)
	}
}
