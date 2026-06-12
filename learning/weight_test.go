package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestWeight(testingTB *testing.T) {
	Convey("Given Weight constructor", testingTB, func() {
		trustWeight := Weight()

		Convey("It should return a usable dynamic", func() {
			So(trustWeight, ShouldNotBeNil)
		})
	})
}

func TestTrustWeight_Observe(testingTB *testing.T) {
	Convey("Given a fresh trust weight", testingTB, func() {
		trustWeight := Weight()

		Convey("When bootstrapping", func() {
			value := trustWeight.Observe(core.Float64(10), core.Float64(10))

			Convey("It should return full trust", func() {
				So(value, ShouldEqual, 1)
			})
		})
	})

	Convey("Given diverging outcomes", testingTB, func() {
		trustWeight := Weight()
		trustWeight.Observe(core.Float64(10), core.Float64(10))
		value := trustWeight.Observe(core.Float64(20), core.Float64(30))

		Convey("It should reduce trust", func() {
			So(float64(value), ShouldBeLessThan, 1)
		})
	})

	Convey("Given zero predicted", testingTB, func() {
		trustWeight := Weight()

		Convey("When observing", func() {
			value := trustWeight.Observe(core.Float64(0), core.Float64(10))

			Convey("It should return ErrZeroPredicted", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})

	Convey("Given invalid stage inputs", testingTB, func() {
		trustWeight := Weight()

		Convey("When observing", func() {
			value := trustWeight.Observe(blankNumber{})

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestTrustWeight_Reset(testingTB *testing.T) {
	Convey("Given trust weight with state", testingTB, func() {
		trustWeight := Weight()
		trustWeight.Observe(core.Float64(10), core.Float64(10))

		Convey("When reset", func() {
			err := trustWeight.Reset()
So(err, ShouldBeNil)

			Convey("It should clear derived state", func() {
				So(trustWeight.state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkWeight_Observe(testingTB *testing.B) {
	trustWeight := Weight()
	trustWeight.Observe(core.Float64(10), core.Float64(10))

	for testingTB.Loop() {
		trustWeight.Observe(core.Float64(10), core.Float64(11))
	}
}

func BenchmarkWeight_ObserveSamples(testingTB *testing.B) {
	trustWeight := Weight()
	predicted := make([]float64, 1024)
	actual := make([]float64, len(predicted))
	out := make([]float64, len(predicted))

	for testingTB.Loop() {
		trustWeight.state.Reset()
		trustWeight.ObserveSamples(predicted, actual, out)
	}
}
