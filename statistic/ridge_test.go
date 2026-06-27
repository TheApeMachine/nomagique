package statistic

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRidgeSolver_Solve(testingTB *testing.T) {
	Convey("Given a well-conditioned normal system", testingTB, func() {
		normal := [][]float64{
			{2, 0.5},
			{0.5, 2},
		}
		vector := []float64{1, 1}
		solver := NewRidgeSolver()

		solution, err := solver.Solve(normal, vector)

		Convey("It should return a finite solution", func() {
			So(err, ShouldBeNil)
			So(len(solution), ShouldEqual, 2)
		})
	})

	Convey("Given a SINGULAR system from a constant predictor", testingTB, func() {
		// Design matrix [1, c] over N rows with constant c: the second column is
		// perfectly collinear with the intercept, so the normal matrix is rank-1
		// and singular. This is the thin-pair case that flooded the log — order
		// flow that never varies. Ridge must rescue it, not fail.
		const (
			rows     = 8.0
			constant = 3.0
		)
		normal := [][]float64{
			{rows, rows * constant},
			{rows * constant, rows * constant * constant},
		}
		vector := []float64{rows * 2, rows * constant * 2}
		solver := NewRidgeSolver()

		solution, err := solver.Solve(normal, vector)

		Convey("It returns a finite damped solution rather than erroring", func() {
			So(err, ShouldBeNil)
			So(len(solution), ShouldEqual, 2)

			for _, value := range solution {
				So(math.IsNaN(value), ShouldBeFalse)
				So(math.IsInf(value, 0), ShouldBeFalse)
			}
		})
	})

	Convey("Given a rank-deficient system with no unregularized escape axis", testingTB, func() {
		normal := [][]float64{
			{4, 4, 0},
			{4, 4, 0},
			{0, 0, 0},
		}
		vector := []float64{2, 2, 0}
		solver := NewRidgeSolver()

		solution, err := solver.Solve(normal, vector)

		Convey("It should regularize every diagonal and return finite coefficients", func() {
			So(err, ShouldBeNil)
			So(solution, ShouldHaveLength, 3)

			for _, value := range solution {
				So(math.IsNaN(value), ShouldBeFalse)
				So(math.IsInf(value, 0), ShouldBeFalse)
			}
		})
	})
}
