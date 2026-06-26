package geometry_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/geometry"
)

func couplingWire(artifact *datura.Artifact, left float64, right float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{"sample", "paired"}, "inputs")
	artifact.Merge("features", []float64{left, right})

	return artifact
}

func velocityWire(artifact *datura.Artifact, sample float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{"sample"}, "inputs")
	artifact.Merge("features", []float64{sample})

	return artifact
}

func TestIntegration(t *testing.T) {
	Convey("Given geometry stages composed through nomagique.Number", t, func() {
		Convey("When Coupling observes co-moving growth", func() {
			artifact := couplingWire(datura.Acquire("test", datura.APPJSON), 2, 2)
			pipeline := nomagique.Number(geometry.NewCoupling(datura.Acquire("coupling-config", datura.APPJSON).
				Poke("sample", "sampleKey").
				Poke("paired", "pairedKey")))
			err := nomagique.RoundTripArtifact(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldAlmostEqual, 1, 1e-9)
		})

		Convey("When Velocity streams consecutive means", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			pipeline := nomagique.Number(geometry.NewVelocity(datura.Acquire("velocity-config", datura.APPJSON).
				Poke("sample", "input")))

			velocityWire(artifact, 1)
			err := nomagique.RoundTripArtifact(artifact, pipeline)

			So(err, ShouldNotBeNil)

			velocityWire(artifact, 1.5)
			err = nomagique.RoundTripArtifact(artifact, pipeline)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldAlmostEqual, 0.5, 1e-12)
		})

		Convey("When Procrustes aligns identical matrices", func() {
			nDim := 4
			nSamples := 6
			matA := [][]float64{
				{1, 0, 0, 0},
				{0, 1, 0, 0},
				{0, 0, 1, 0},
				{1, 1, 0, 0},
				{0, 0, 1, 1},
				{1, 0, 1, 0},
			}
			stage, err := geometry.NewProcrustesFromRows(
				datura.Acquire("procrustes-config", datura.APPJSON),
				matA, matA, nSamples, nDim,
			)
			So(err, ShouldBeNil)

			artifact := datura.Acquire("test", datura.APPJSON)
			err = nomagique.RoundTripArtifact(artifact, nomagique.Number(stage))

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeLessThan, 1e-10)
		})

		Convey("When Rotor and Sandwich run in sequence", func() {
			artifact := datura.Acquire("test", datura.APPJSON)
			rotor := geometry.NewRotor(datura.Acquire("rotor-config", datura.APPJSON))

			artifact.Poke([]float64{0, 1, 0, 0}, "batch")
			err := nomagique.RoundTripArtifact(artifact, rotor)

			So(err, ShouldBeNil)

			motor := datura.Peek[[]float64](artifact, "output", "motor")
			sandwich := geometry.NewSandwich(
				datura.Acquire("sandwich-config", datura.APPJSON).Poke(motor, "motor"),
			)

			artifact.Poke([]float64{1, 0, 0, 0, 0, 0, 0, 0}, "batch")
			err = nomagique.RoundTripArtifact(artifact, nomagique.Number(sandwich))

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)
		})
	})
}
