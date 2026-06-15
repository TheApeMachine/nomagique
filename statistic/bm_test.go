package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBivariateMoment_Observe(testingTB *testing.T) {
	Convey("Given aligned streams", testingTB, func() {
		bivariateMoment := NewBivariateMoment(
			1.0, 1.0,
			[]float64{1, 2, 3, 4},
			[]float64{2, 5, 7, 10},
			nil,
		)
		got := bivariateMoment.Observe()

		Convey("It should return the expected central mixed moment", func() {
			So(float64(got), ShouldEqual, 3.25)
		})
	})

	errorCases := []struct {
		name string
		x    []float64
		y    []float64
	}{
		{"too few samples", []float64{1}, []float64{2}},
		{"length mismatch", []float64{1, 2, 3}, []float64{1, 2}},
	}

	for _, testCase := range errorCases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			bivariateMoment := NewBivariateMoment(
				1.0, 1.0, testCase.x, testCase.y, nil,
			)

			Convey("It should leave output at zero", func() {
				So(float64(bivariateMoment.Observe()), ShouldEqual, 0)
			})
		})
	}

	Convey("Given mismatched weights", testingTB, func() {
		bivariateMoment := NewBivariateMoment(
			1.0, 1.0,
			[]float64{1, 2, 3},
			[]float64{2, 4, 6},
			[]float64{1, 1},
		)

		Convey("It should leave output at zero", func() {
			So(float64(bivariateMoment.Observe()), ShouldEqual, 0)
		})
	})
}

func TestBivariateMoment_Powers(testingTB *testing.T) {
	Convey("Given exponent configuration", testingTB, func() {
		bivariateMoment := NewBivariateMoment(
			2.0, 1.0,
			[]float64{1, 2, 3},
			[]float64{2, 4, 6},
			nil,
		)

		rPower, sPower := bivariateMoment.Powers()

		Convey("It should expose configured powers", func() {
			So(rPower, ShouldEqual, 2)
			So(sPower, ShouldEqual, 1)
		})
	})
}

func TestBivariateMoment_Reset(testingTB *testing.T) {
	Convey("Given an observed bivariate moment", testingTB, func() {
		bivariateMoment := NewBivariateMoment(
			1.0, 1.0,
			[]float64{1, 2, 3},
			[]float64{2, 4, 6},
			[]float64{1, 1, 1},
		)
		_ = bivariateMoment.Observe()

		So(bivariateMoment.Reset(), ShouldBeNil)

		Convey("It should clear weights", func() {
			So(bivariateMoment.weights, ShouldBeNil)
		})
	})
}

func BenchmarkBivariateMoment_Observe(b *testing.B) {
	bivariateMoment := NewBivariateMoment(
		2.0, 1.0,
		[]float64{1, 2, 3, 4, 5, 6},
		[]float64{2, 4, 6, 8, 10, 12},
		nil,
	)

	b.ReportAllocs()

	for b.Loop() {
		_ = bivariateMoment.Observe()
	}
}
