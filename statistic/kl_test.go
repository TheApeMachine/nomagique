package statistic

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
)

func TestKLDivergence_Observe(testingTB *testing.T) {
	Convey("Given matching observed and expected halves", testingTB, func() {
		kl := NewKLDivergence(nil, 0, 0)
		inputs := append(
			nomagique.Numbers(1, 1),
			nomagique.Numbers(1, 1)...,
		)
		value := kl.Observe(inputs...)

		Convey("It should return zero divergence", func() {
			So(float64(value), ShouldEqual, 0)
		})
	})

	Convey("Given fewer than two inputs", testingTB, func() {
		value := NewKLDivergence(nil, 0, 0).Observe(nomagique.Numbers(1)...)

		Convey("It should return zero", func() {
			So(float64(value), ShouldEqual, 0)
		})
	})

	Convey("Given an odd input count", testingTB, func() {
		value := NewKLDivergence(nil, 0, 0).Observe(
			nomagique.Numbers(1, 1, 1)...,
		)

		Convey("It should return zero", func() {
			So(float64(value), ShouldEqual, 0)
		})
	})

	Convey("Given a non-finite observed sample", testingTB, func() {
		inputs := append(
			[]core.Number{core.Float64(1), core.Float64(math.NaN())},
			nomagique.Numbers(1, 1)...,
		)
		value := NewKLDivergence(nil, 0, 0).Observe(inputs...)

		Convey("It should return zero", func() {
			So(float64(value), ShouldEqual, 0)
		})
	})

	Convey("Given batch halves wired through Observe", testingTB, func() {
		inputs := append(
			nomagique.Numbers(0.25, 0.25, 0.25, 0.25),
			nomagique.Numbers(0.25, 0.25, 0.25, 0.25)...,
		)
		value := NewKLDivergence(nil, 1, 1e-6).Observe(inputs...)

		Convey("It should return zero divergence", func() {
			So(float64(value), ShouldAlmostEqual, 0, 1e-6)
		})
	})
}

func BenchmarkKLDivergence_Observe(testingTB *testing.B) {
	kl := NewKLDivergence(nil, 0, 0)
	inputs := append(
		nomagique.Numbers(1, 2, 3, 4),
		nomagique.Numbers(1, 1, 2, 4)...,
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = kl.Observe(inputs...)
	}
}
