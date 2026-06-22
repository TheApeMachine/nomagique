package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestGeometricMeanRead(testingTB *testing.T) {
	Convey("Given two positive output fields", testingTB, func() {
		config := datura.Acquire("geometric-mean-config", datura.APPJSON).
			Poke(map[string]any{
				"leftKey":        "left",
				"rightKey":       "right",
				"destinationKey": "joint",
			}, "joint")

		stage := NewGeometricMean(config)
		artifact := datura.Acquire("geometric-mean-test", datura.APPJSON)
		artifact.Poke(4.0, "output", "left")
		artifact.Poke(9.0, "output", "right")

		err := transport.NewFlipFlop(artifact, stage)

		So(err, ShouldBeNil)

		Convey("It should publish their geometric mean", func() {
			So(datura.Peek[float64](artifact, "output", "joint"), ShouldEqual, 6)
		})
	})

	Convey("Given a non-positive operand", testingTB, func() {
		config := datura.Acquire("geometric-mean-config", datura.APPJSON).
			Poke(map[string]any{
				"leftKey":        "left",
				"rightKey":       "right",
				"destinationKey": "joint",
			}, "joint")

		stage := NewGeometricMean(config)
		artifact := datura.Acquire("geometric-mean-test", datura.APPJSON)
		artifact.Poke(4.0, "output", "left")
		artifact.Poke(0.0, "output", "right")

		err := transport.NewFlipFlop(artifact, stage)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkGeometricMeanRead(testingTB *testing.B) {
	config := datura.Acquire("geometric-mean-config-bench", datura.APPJSON).
		Poke(map[string]any{
			"leftKey":        "left",
			"rightKey":       "right",
			"destinationKey": "joint",
		}, "joint")

	stage := NewGeometricMean(config)
	artifact := datura.Acquire("geometric-mean-bench", datura.APPJSON)
	artifact.Poke(4.0, "output", "left")
	artifact.Poke(9.0, "output", "right")

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, stage)
	}
}
