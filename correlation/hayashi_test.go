package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func hayashiConfig() *datura.Artifact {
	return datura.Acquire("hayashi-config", datura.APPJSON).
		Poke(1, "config", "maxIntervalSeconds")
}

func TestHayashiYoshidaRead(testingTB *testing.T) {
	Convey("Given proportional async streams", testingTB, func() {
		hayashi := NewHayashiYoshida(hayashiConfig())
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke([]float64{0, 100, 1, 110, 0, 50, 1, 55}, "batch")
		err := transport.NewFlipFlop(artifact, hayashi)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should estimate correlation near one", func() {
			So(got, ShouldAlmostEqual, 1, 1e-9)
		})
	})

	Convey("Given empty Observe inputs", testingTB, func() {
		hayashi := NewHayashiYoshida(hayashiConfig())
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, hayashi)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given fewer than two inputs", testingTB, func() {
		hayashi := NewHayashiYoshida(hayashiConfig())
		artifact := datura.Acquire("test", datura.APPJSON).Poke(1, "sample")
		err := transport.NewFlipFlop(artifact, hayashi)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given odd input count", testingTB, func() {
		hayashi := NewHayashiYoshida(hayashiConfig())
		artifact := datura.Acquire("test", datura.APPJSON).Poke([]float64{0, 100, 1}, "batch")
		err := transport.NewFlipFlop(artifact, hayashi)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a half that is not time-value pairs", testingTB, func() {
		hayashi := NewHayashiYoshida(hayashiConfig())
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke([]float64{0, 100, 0, 50, 1, 55}, "batch")
		err := transport.NewFlipFlop(artifact, hayashi)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestHayashiYoshida_Reset(testingTB *testing.T) {
	Convey("Given an observed Hayashi stage", testingTB, func() {
		hayashi := NewHayashiYoshida(hayashiConfig())
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke([]float64{0, 100, 1, 110, 0, 50, 1, 55}, "batch")
		err := transport.NewFlipFlop(artifact, hayashi)

		So(err, ShouldBeNil)

		resetArtifact := datura.Acquire("test", datura.APPJSON).Poke(1, "reset")
		err = transport.NewFlipFlop(resetArtifact, hayashi)

		So(err, ShouldNotBeNil)

		fresh := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(fresh, hayashi)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkHayashiYoshidaRead(testingTB *testing.B) {
	hayashi := NewHayashiYoshida(hayashiConfig())
	artifact := datura.Acquire("test", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact.Poke([]float64{
			0, 100, 1, 110, 2, 121, 3, 133.1,
			0, 50, 1, 55, 2, 60.5, 3, 66.55,
		}, "batch")
		_ = transport.NewFlipFlop(artifact, hayashi)
	}
}
