//go:build !darwin || !cgo

package manifold

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewSolver_stub(testingTB *testing.T) {
	Convey("Given the stub resonance solver build", testingTB, func() {
		solver, err := NewSolver([]int{4, 8, 4}, 2, 0.01)

		Convey("It should refuse to start without Metal", func() {
			So(solver, ShouldBeNil)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestSolverStub_methods(testingTB *testing.T) {
	Convey("Given a zero-value stub solver", testingTB, func() {
		solver := &Solver{}

		Convey("It should error on runtime operations", func() {
			So(solver.ResetState(false), ShouldNotBeNil)
			So(solver.Settle([]float64{0.1}, true), ShouldNotBeNil)
			So(solver.Learn(nil), ShouldNotBeNil)

			_, energyErr := solver.Energy()
			So(energyErr, ShouldNotBeNil)
		})
	})
}
