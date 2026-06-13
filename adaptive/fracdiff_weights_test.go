package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFracDiffOrder(testingTB *testing.T) {
	Convey("Given zero rate with span", testingTB, func() {
		order := fracDiffOrder(0, 9)

		Convey("It should derive an order inside the unit interval", func() {
			So(order, ShouldBeGreaterThan, 0)
			So(order, ShouldBeLessThan, 1)
		})
	})

	Convey("Given unit rate with span", testingTB, func() {
		order := fracDiffOrder(1, 9)

		Convey("It should clamp below one", func() {
			So(order, ShouldBeLessThan, 1)
		})
	})
}

func TestBuildFracDiffWeights(testingTB *testing.T) {
	Convey("Given a fractional order", testingTB, func() {
		weights, width := buildFracDiffWeights(0.4, 10, 5, nil)

		Convey("It should start with unit weight on the newest lag", func() {
			So(width, ShouldBeGreaterThan, 1)
			So(weights[0], ShouldEqual, 1)
			So(weights[1], ShouldAlmostEqual, -0.4, 1e-12)
		})
	})
}

func TestFracDiffWeightThreshold(testingTB *testing.T) {
	Convey("Given zero span and positive reference", testingTB, func() {
		Convey("It should derive the floor from reference", func() {
			So(fracDiffWeightThreshold(0, 5), ShouldEqual, 0.2)
		})
	})

	Convey("Given zero span and zero reference", testingTB, func() {
		Convey("It should fall back to unity", func() {
			So(fracDiffWeightThreshold(0, 0), ShouldEqual, 1)
		})
	})
}

func TestFracDiffMaxLag(testingTB *testing.T) {
	Convey("Given sub-unit span", testingTB, func() {
		Convey("It should keep at least one lag", func() {
			So(fracDiffMaxLag(0.5), ShouldEqual, 1)
		})
	})

	Convey("Given a ten-wide span", testingTB, func() {
		Convey("It should cap the tail from range", func() {
			So(fracDiffMaxLag(10), ShouldEqual, 11)
		})
	})
}

func BenchmarkBuildFracDiffWeights(testingTB *testing.B) {
	scratch := make([]float64, 0, 32)

	for testingTB.Loop() {
		_, _ = buildFracDiffWeights(0.35, 20, 10, scratch[:0])
	}
}
