package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestCovariance_Observe(testingTB *testing.T) {
	Convey("Given positively coupled streams", testingTB, func() {
		covariance := NewCovariance(nil)
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke([]float64{1, 2, 3, 4, 2, 4, 6, 8}, "batch")
		err := transport.NewFlipFlop(artifact, covariance)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return positive covariance", func() {
			So(got, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given empty Observe inputs", testingTB, func() {
		covariance := NewCovariance(nil)
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, covariance)

		So(err, ShouldBeNil)

		Convey("It should return zero output", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestCovariance_Reset(testingTB *testing.T) {
	Convey("Given an observed covariance stage", testingTB, func() {
		covariance := NewCovariance(nil)
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke([]float64{1, 2, 3, 4, 2, 4, 6, 8}, "batch")
		err := transport.NewFlipFlop(artifact, covariance)

		So(err, ShouldBeNil)
		So(covariance.Reset(), ShouldBeNil)

		fresh := datura.Acquire("test", datura.APPJSON)
		err = transport.NewFlipFlop(fresh, covariance)

		So(err, ShouldBeNil)

		Convey("It should clear output", func() {
			So(datura.Peek[float64](fresh, "output", "value"), ShouldEqual, 0)
		})
	})
}

func BenchmarkCovariance_Observe(testingTB *testing.B) {
	covariance := NewCovariance(nil)
	artifact := datura.Acquire("test", datura.APPJSON)

	for testingTB.Loop() {
		artifact.Poke([]float64{1, 2, 3, 4, 5, 6, 7, 8, 2, 4, 6, 8, 10, 12, 14, 16}, "batch")
		_ = transport.NewFlipFlop(artifact, covariance)
	}
}
