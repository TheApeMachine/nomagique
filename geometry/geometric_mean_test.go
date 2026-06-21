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
			}, "inputs", "joint")

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
}
