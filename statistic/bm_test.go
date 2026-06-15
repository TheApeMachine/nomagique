package statistic

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBivariateMoment_Observe(testingTB *testing.T) {
	Convey("Given aligned streams", testingTB, func() {
		bivariateMoment := NewBivariateMoment(1.0, 1.0, nil)
		got := observeInputs(bivariateMoment, 1, 2, 3, 4, 2, 5, 7, 10)

		Convey("It should return the expected central mixed moment", func() {
			So(got, ShouldEqual, 3.25)
		})
	})

	errorCases := []struct {
		name    string
		samples []float64
	}{
		{"too few samples", []float64{1, 2}},
		{"length mismatch", []float64{1, 2, 3, 1, 2}},
	}

	for _, testCase := range errorCases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			bivariateMoment := NewBivariateMoment(1.0, 1.0, nil)

			Convey("It should leave output at zero", func() {
				So(observeInputs(bivariateMoment, testCase.samples...), ShouldEqual, 0)
			})
		})
	}

	Convey("Given mismatched weights", testingTB, func() {
		bivariateMoment := NewBivariateMoment(1.0, 1.0, []float64{1, 1})

		Convey("It should leave output at zero", func() {
			So(observeInputs(bivariateMoment, 1, 2, 3, 2, 4, 6), ShouldEqual, 0)
		})
	})
}

func TestBivariateMoment_Powers(testingTB *testing.T) {
	Convey("Given exponent configuration", testingTB, func() {
		bivariateMoment := NewBivariateMoment(2.0, 1.0, nil)

		rPower, sPower := bivariateMoment.Powers()

		Convey("It should expose configured powers", func() {
			So(rPower, ShouldEqual, 2)
			So(sPower, ShouldEqual, 1)
		})
	})
}

func TestBivariateMoment_Reset(testingTB *testing.T) {
	Convey("Given an observed bivariate moment", testingTB, func() {
		bivariateMoment := NewBivariateMoment(1.0, 1.0, []float64{1, 1, 1})
		_ = observeInputs(bivariateMoment, 1, 2, 3, 2, 4, 6)

		So(bivariateMoment.Reset(), ShouldBeNil)

		Convey("It should clear weights", func() {
			So(bivariateMoment.weights, ShouldBeNil)
		})
	})
}

func BenchmarkBivariateMoment_Observe(b *testing.B) {
	bivariateMoment := NewBivariateMoment(2.0, 1.0, nil)

	b.ReportAllocs()

	for b.Loop() {
		_ = observeInputs(bivariateMoment, 1, 2, 3, 4, 5, 6, 2, 4, 6, 8, 10, 12)
	}
}
