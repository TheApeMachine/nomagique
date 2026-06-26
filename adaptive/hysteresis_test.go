package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func hysteresisWire(sample float64, window int) *datura.Artifact {
	artifact := datura.Acquire("hysteresis-wire", datura.APPJSON)
	artifact.Poke("features", "root")
	artifact.Poke([]string{"sample"}, "inputs")
	artifact.Merge("features", []float64{sample})
	artifact.Poke(float64(window), "window")

	return artifact
}

func TestHysteresis_Read(testingTB *testing.T) {
	Convey("Given a hysteresis stage", testingTB, func() {
		Convey("It should require consecutive high samples before switching on", func() {
			stage := NewHysteresis(datura.Acquire("hysteresis-config", datura.APPJSON).
				Poke("sample", "input").
				Poke(float64(3), "window"))

			for range 2 {
				artifact := hysteresisWire(1.0, 3)
				err := nomagique.RoundTripArtifact(artifact, stage)

				So(err, ShouldBeNil)
				So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
				artifact.Release()
			}

			artifact := hysteresisWire(1.0, 3)
			err := nomagique.RoundTripArtifact(artifact, stage)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)
			artifact.Release()
		})

		Convey("It should treat magnitudes above threshold as high", func() {
			thresholdStage := NewHysteresis(datura.Acquire("hysteresis-threshold", datura.APPJSON).
				Poke("sample", "input").
				Poke(0.5, "threshold").
				Poke(float64(3), "window"))

			for range 2 {
				artifact := hysteresisWire(0.75, 3)
				err := nomagique.RoundTripArtifact(artifact, thresholdStage)

				So(err, ShouldBeNil)
				So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
				artifact.Release()
			}

			artifact := hysteresisWire(0.75, 3)
			err := nomagique.RoundTripArtifact(artifact, thresholdStage)

			So(err, ShouldBeNil)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)
			artifact.Release()
		})
	})
}

func BenchmarkHysteresis_Read(b *testing.B) {
	stage := NewHysteresis(datura.Acquire("hysteresis-config", datura.APPJSON).
		Poke("sample", "input").
		Poke(float64(2), "window"))

	b.ReportAllocs()

	for b.Loop() {
		artifact := hysteresisWire(1.0, 2)
		_ = nomagique.RoundTripArtifact(artifact, stage)
		artifact.Release()
	}
}
