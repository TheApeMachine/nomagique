package fluid

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestGridCIC(t *testing.T) {
	Convey("Given a particle crossing the periodic domain boundary", t, func() {
		grid := Grid{X: 2, Y: 2, Z: 2, Spacing: 0.5}
		inside, err := grid.CIC(Vector{X: 0.75, Y: 0.25, Z: 0.5})
		wrapped, wrappedErr := grid.CIC(Vector{X: -0.25, Y: 1.25, Z: 1.5})

		Convey("It should produce the same periodic stencil", func() {
			So(err, ShouldBeNil)
			So(wrappedErr, ShouldBeNil)
			So(wrapped.Indices, ShouldResemble, inside.Indices)
			So(wrapped.Weights, ShouldResemble, inside.Weights)
		})

		Convey("It should partition one particle exactly across its corners", func() {
			var total float32

			for _, weight := range inside.Weights {
				total += weight
			}

			So(total, ShouldAlmostEqual, 1.0)
		})
	})
}

func TestConservedGridScatter(t *testing.T) {
	Convey("Given particles with known mass, momentum, and energy", t, func() {
		grid := Grid{X: 2, Y: 2, Z: 2, Spacing: 0.5}
		state, err := NewConservedGrid(grid)
		So(err, ShouldBeNil)
		positions := []Vector{{X: 0.25, Y: 0.25, Z: 0.25}, {X: 0.75, Y: 0.75, Z: 0.75}}
		velocities := []Vector{{X: 2}, {Y: -1}}
		masses := []float32{2, 3}
		internal := []float32{5, 7}

		err = state.Scatter(grid, positions, velocities, masses, internal)

		Convey("It should conserve the particle totals after restoring cell volume", func() {
			So(err, ShouldBeNil)
			volume := grid.Spacing * grid.Spacing * grid.Spacing
			var mass, momentumX, momentumY, energy float32

			for cell := range state.Density {
				mass += state.Density[cell] * volume
				momentumX += state.Momentum[cell].X * volume
				momentumY += state.Momentum[cell].Y * volume
				energy += state.Energy[cell] * volume
			}

			So(mass, ShouldAlmostEqual, 5.0)
			So(momentumX, ShouldAlmostEqual, 4.0)
			So(momentumY, ShouldAlmostEqual, -3.0)
			So(energy, ShouldAlmostEqual, 17.5)
		})
	})
}
