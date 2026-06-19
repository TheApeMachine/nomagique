package correlation

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestHayashiYoshida_Observe(testingTB *testing.T) {
	Convey("Given proportional async streams", testingTB, func() {
		hayashi := NewHayashiYoshida(nil, time.Second)
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
		hayashi := NewHayashiYoshida(nil, time.Second)
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, hayashi)

		So(err, ShouldBeNil)

		Convey("It should return zero output", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
		})
	})

	Convey("Given fewer than two inputs", testingTB, func() {
		hayashi := NewHayashiYoshida(nil, time.Second)
		artifact := datura.Acquire("test", datura.APPJSON).Poke(1, "sample")
		err := transport.NewFlipFlop(artifact, hayashi)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return zero", func() {
			So(got, ShouldEqual, 0)
		})
	})

	Convey("Given odd input count", testingTB, func() {
		hayashi := NewHayashiYoshida(nil, time.Second)
		artifact := datura.Acquire("test", datura.APPJSON).Poke([]float64{0, 100, 1}, "batch")
		err := transport.NewFlipFlop(artifact, hayashi)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return zero", func() {
			So(got, ShouldEqual, 0)
		})
	})

	Convey("Given a half that is not time-value pairs", testingTB, func() {
		hayashi := NewHayashiYoshida(nil, time.Second)
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke([]float64{0, 100, 0, 50, 1, 55}, "batch")
		err := transport.NewFlipFlop(artifact, hayashi)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return zero", func() {
			So(got, ShouldEqual, 0)
		})
	})
}

func TestHayashiYoshida_Reset(testingTB *testing.T) {
	Convey("Given an observed Hayashi stage", testingTB, func() {
		hayashi := NewHayashiYoshida(nil, time.Second)
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke([]float64{0, 100, 1, 110, 0, 50, 1, 55}, "batch")
		err := transport.NewFlipFlop(artifact, hayashi)

		So(err, ShouldBeNil)
		So(hayashi.Reset(), ShouldBeNil)

		fresh := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(fresh, hayashi)

		So(err, ShouldBeNil)

		Convey("It should clear output", func() {
			So(datura.Peek[float64](fresh, "output", "value"), ShouldEqual, 0)
		})
	})
}

func BenchmarkHayashiYoshida_Observe(testingTB *testing.B) {
	hayashi := NewHayashiYoshida(nil, time.Second)
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
