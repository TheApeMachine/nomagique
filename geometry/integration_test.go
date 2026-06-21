package geometry_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/geometry"
)

func TestIntegration(t *testing.T) {
	Convey("Given geometry stages composed through nomagique.Number", t, func() {
		Convey("When Coupling observes co-moving growth", func() {
			artifact := datura.Acquire("test", datura.APPJSON).
				Poke(2, "sample").
				Poke(2, "paired")
			pipeline := nomagique.Number(geometry.NewCoupling(datura.Acquire("coupling-config", datura.APPJSON)))
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldAlmostEqual, 1, 1e-9)
		})

		Convey("When Velocity streams consecutive means", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			pipeline := nomagique.Number(geometry.NewVelocity(datura.Acquire("velocity-config", datura.APPJSON)))

			artifact.Poke(1, "sample")
			err := transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)

			artifact.Poke(1.5, "sample")
			err = transport.NewFlipFlop(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldAlmostEqual, 0.5, 1e-12)
		})

		Convey("When Rotor and Sandwich run in sequence", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			rotor := geometry.NewRotor(datura.Acquire("rotor-config", datura.APPJSON))

			artifact.Poke([]float64{0, 1, 0, 0}, "batch")
			err := transport.NewFlipFlop(artifact, rotor)

			So(err, ShouldBeNil)

			sandwich := geometry.NewSandwich(
				datura.Acquire("sandwich-config", datura.APPJSON),
				rotor.Multivector(),
			)
			err = transport.NewFlipFlop(artifact, nomagique.Number(sandwich))

			So(err, ShouldBeNil)

			artifact.Poke([]float64{1, 0, 0, 0, 0, 0, 0, 0}, "batch")
			err = transport.NewFlipFlop(artifact, nomagique.Number(sandwich))

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)
		})
	})
}
