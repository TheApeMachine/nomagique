package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"gonum.org/v1/gonum/mat"
)

func TestDenseFill(testingTB *testing.T) {
	Convey("Given a dense column", testingTB, func() {
		column := mat.NewDense(3, 1, nil)

		denseFill(column, 2.5)

		Convey("It should fill every element", func() {
			So(mat.Equal(column, mat.NewDense(3, 1, []float64{2.5, 2.5, 2.5})), ShouldBeTrue)
		})
	})
}

func TestDenseVarianceEMAInto(testingTB *testing.T) {
	Convey("Given retained variances and a residual column", testingTB, func() {
		variance := mat.NewDense(3, 1, []float64{1, 2, 3})
		residual := mat.NewDense(3, 1, []float64{2, 0, -4})
		scratch := mat.NewDense(3, 1, nil)

		denseVarianceEMAInto(variance, residual, scratch, 0.25, 1)

		Convey("It should update the whole column and apply the floor", func() {
			expected := mat.NewDense(3, 1, []float64{1.75, 1.5, 6.25})
			So(mat.EqualApprox(variance, expected, 1e-15), ShouldBeTrue)
		})
	})
}

func TestDensePrecisionFromVarianceInto(testingTB *testing.T) {
	Convey("Given a variance column spanning both precision bounds", testingTB, func() {
		variance := mat.NewDense(3, 1, []float64{0.01, 2, 100})
		precision := mat.NewDense(3, 1, nil)

		densePrecisionFromVarianceInto(precision, variance, 0.1, 5)

		Convey("It should invert and clamp the whole column", func() {
			expected := mat.NewDense(3, 1, []float64{5, 0.5, 0.1})
			So(mat.EqualApprox(precision, expected, 1e-15), ShouldBeTrue)
		})
	})
}
