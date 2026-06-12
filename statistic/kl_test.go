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
		kl := NewKLDivergence(nil)
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
		value := NewKLDivergence(nil).Observe(nomagique.Numbers(1)...)

		Convey("It should return zero", func() {
			So(float64(value), ShouldEqual, 0)
		})
	})

	Convey("Given an odd input count", testingTB, func() {
		value := NewKLDivergence(nil).Observe(
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
		value := NewKLDivergence(nil).Observe(inputs...)

		Convey("It should return zero", func() {
			So(float64(value), ShouldEqual, 0)
		})
	})
}

func BenchmarkKLDivergence_Observe(testingTB *testing.B) {
	kl := NewKLDivergence(nil)
	inputs := append(
		nomagique.Numbers(1, 2, 3, 4),
		nomagique.Numbers(1, 1, 2, 4)...,
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = kl.Observe(inputs...)
	}
}
