//go:build !darwin || !cgo

package manifold

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewBatchSolver_stub(testingTB *testing.T) {
	Convey("Given the stub batch solver build", testingTB, func() {
		solver, err := NewBatchSolver([]int{4, 8, 4}, 2, 16, 0.01)

		Convey("It should refuse to start without Metal", func() {
			So(solver, ShouldBeNil)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestBatchSolverStub_methods(testingTB *testing.T) {
	Convey("Given a zero-value stub solver", testingTB, func() {
		solver := &BatchSolver{}

		Convey("It should error on runtime operations", func() {
			So(solver.ResetState(false), ShouldNotBeNil)
			So(solver.SetInput(0, []float64{0.1}, nil), ShouldNotBeNil)
			So(solver.Settle(true), ShouldNotBeNil)
			So(solver.Learn(), ShouldNotBeNil)

			_, energyErr := solver.Energy(0)
			So(energyErr, ShouldNotBeNil)
		})
	})
}
