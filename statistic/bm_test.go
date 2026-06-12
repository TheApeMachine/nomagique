package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestBivariateMoment_Observe(testingTB *testing.T) {
	Convey("Given aligned x and y streams", testingTB, func() {
		value := NewBivariateMoment(
			1, 1,
			nomagique.Numbers(1, 2, 3),
			nomagique.Numbers(2, 4, 6),
			nil,
		).Observe()

		Convey("It should return the sample covariance", func() {
			So(float64(value), ShouldAlmostEqual, 4.0/3.0, 1e-9)
		})
	})

	Convey("Given mismatched stream lengths", testingTB, func() {
		value := NewBivariateMoment(
			1, 1,
			nomagique.Numbers(1, 2, 3),
			nomagique.Numbers(2, 4),
			nil,
		).Observe()

		Convey("It should return zero", func() {
			So(float64(value), ShouldEqual, 0)
		})
	})
}

func BenchmarkBivariateMoment_Observe(testingTB *testing.B) {
	bivariateMoment := NewBivariateMoment(
		1, 1,
		nomagique.Numbers(1, 2, 3, 4, 5, 6),
		nomagique.Numbers(2, 4, 6, 8, 10, 12),
		nil,
	)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = bivariateMoment.Observe()
	}
}
