package logic

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/tests"
)

func pipelineValue(stage io.ReadWriter, sample float64) float64 {
	value, _ := tests.PipelineSample([]io.ReadWriter{stage}, sample)

	return value
}

func TestNewCircuit(testingTB *testing.T) {
	Convey("Given NewCircuit", testingTB, func() {
		circuit := NewCircuit(Rules{
			{
				Condition: True{Operand: true},
				Then:      NewConstant(1),
			},
		})

		Convey("It should return a wired circuit", func() {
			So(circuit, ShouldNotBeNil)
		})
	})
}

func TestCircuit_Observe(testingTB *testing.T) {
	Convey("Given a carried signal above its threshold", testingTB, func() {
		consequence := adaptive.NewEMA()
		alternative := adaptive.NewEMA()

		circuit := NewCircuit(Rules{
			{
				Condition: GreaterThan{
					Right: NewConstant(2),
				},
				Then: consequence,
			},
			{
				Condition: True{Operand: true},
				Then:      alternative,
			},
		})

		above := pipelineCircuit(circuit, 3)

		_ = circuit.Reset()

		below := pipelineCircuit(circuit, 1)

		Convey("It should route through the matching branch", func() {
			So(above, ShouldEqual, pipelineValue(consequence, 3))
			So(below, ShouldEqual, pipelineValue(alternative, 1))
		})
	})
}

func TestCircuit_ObserveAnd(testingTB *testing.T) {
	Convey("Given a compound And condition", testingTB, func() {
		circuit := NewCircuit(Rules{
			{
				Condition: And{
					GreaterThan{
						Right: NewConstant(2),
					},
					True{
						Stage: NewConstant(1),
					},
				},
				Then: NewConstant(10),
			},
			{
				Condition: True{Operand: true},
				Then:      NewConstant(20),
			},
		})

		Convey("It should require every nested condition", func() {
			So(pipelineCircuit(circuit, 3), ShouldEqual, 10)
		})
	})
}

func TestCircuit_Reset(testingTB *testing.T) {
	Convey("Given an observed circuit", testingTB, func() {
		circuit := NewCircuit(Rules{
			{
				Condition: True{Operand: true},
				Then:      NewConstant(7),
			},
		})
		_ = pipelineCircuit(circuit, 0)

		Convey("It should reset without error", func() {
			So(circuit.Reset(), ShouldBeNil)
			So(pipelineCircuit(circuit, 0), ShouldEqual, 7)
		})
	})
}

func BenchmarkCircuit_Observe(benchmark *testing.B) {
	circuit := NewCircuit(Rules{
		{
			Condition: GreaterThan{
				Right: NewConstant(2),
			},
			Then: adaptive.NewEMA(),
		},
		{
			Condition: True{Operand: true},
			Then:      NewConstant(0),
		},
	})

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		_ = pipelineCircuit(circuit, 3)
	}
}
