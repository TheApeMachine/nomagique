package fluid

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func gasFixture() (*Gas, *GasState) {
	grid := Grid{X: 4, Y: 4, Z: 4, Spacing: 0.25}
	material := GasMaterial{
		Gamma:               1.4,
		SpecificGasConstant: 1,
		SpecificHeat:        3.5,
	}
	gas, err := NewGas(grid, material, DefaultGasNumerics(), nil)

	if err != nil {
		panic(err)
	}

	state, err := NewGasState(grid)

	if err != nil {
		panic(err)
	}

	velocity := Vector{X: 0.2, Y: -0.1, Z: 0.05}
	kinetic := 0.5 * (velocity.X*velocity.X + velocity.Y*velocity.Y + velocity.Z*velocity.Z)

	for cell := range state.Density {
		state.Density[cell] = 1
		state.Momentum[cell] = velocity
		state.Energy[cell] = 1/(material.Gamma-1) + kinetic
	}

	return gas, state
}

func TestGasStableDelta(t *testing.T) {
	Convey("Given a uniform periodic ideal gas", t, func() {
		gas, state := gasFixture()

		delta, err := gas.StableDelta(state)

		Convey("It should derive the unsplit three-dimensional CFL limit", func() {
			So(err, ShouldBeNil)
			sound := float32(math.Sqrt(1.4))
			expected := float32(0.4) * 0.25 / (0.35 + 3*sound)
			So(delta, ShouldAlmostEqual, expected, 1e-7)
		})
	})
}

func TestGasAdvance(t *testing.T) {
	Convey("Given the total-energy CPU gas reference", t, func() {
		gas, state := gasFixture()
		delta, err := gas.StableDelta(state)
		So(err, ShouldBeNil)

		Convey("It should leave a uniform periodic state invariant", func() {
			next, advanceErr := gas.Advance(state, delta)
			So(advanceErr, ShouldBeNil)
			So(next.Density, ShouldResemble, state.Density)
			So(next.Momentum, ShouldResemble, state.Momentum)
			So(next.Energy, ShouldResemble, state.Energy)
		})

		Convey("It should conservatively spread a localized pressure pulse", func() {
			state.Energy[21] += 1
			before := gasTotals(state)
			pulseDelta, stableErr := gas.StableDelta(state)
			So(stableErr, ShouldBeNil)

			next, advanceErr := gas.Advance(state, 0.5*pulseDelta)
			So(advanceErr, ShouldBeNil)
			after := gasTotals(next)

			for component := range before {
				So(after[component], ShouldAlmostEqual, before[component], 1e-4)
			}

			for cell := range next.Density {
				So(next.Density[cell], ShouldBeGreaterThanOrEqualTo, gas.numerics.DensityMinimum)
				kinetic := 0.5 * (next.Momentum[cell].X*next.Momentum[cell].X +
					next.Momentum[cell].Y*next.Momentum[cell].Y +
					next.Momentum[cell].Z*next.Momentum[cell].Z) / next.Density[cell]
				pressure := (gas.material.Gamma - 1) * (next.Energy[cell] - kinetic)
				So(pressure, ShouldBeGreaterThanOrEqualTo, gas.numerics.PressureMinimum)
			}
		})

		Convey("It should project vacuum into the documented admissible set", func() {
			vacuum, stateErr := NewGasState(gas.grid)
			So(stateErr, ShouldBeNil)

			next, advanceErr := gas.Advance(vacuum, delta)
			So(advanceErr, ShouldBeNil)

			for cell := range next.Density {
				So(next.Density[cell], ShouldEqual, gas.numerics.DensityMinimum)
				pressure := (gas.material.Gamma - 1) * next.Energy[cell]
				So(pressure, ShouldAlmostEqual, gas.numerics.PressureMinimum, 1e-7)
			}
		})
	})
}

func BenchmarkGasAdvance(b *testing.B) {
	gas, state := gasFixture()
	delta, err := gas.StableDelta(state)

	if err != nil {
		b.Fatal(err)
	}

	for b.Loop() {
		state, err = gas.Advance(state, delta)

		if err != nil {
			b.Fatal(err)
		}
	}
}

func gasTotals(state *GasState) [5]float64 {
	var totals [5]float64

	for cell := range state.Density {
		totals[0] += float64(state.Density[cell])
		totals[1] += float64(state.Momentum[cell].X)
		totals[2] += float64(state.Momentum[cell].Y)
		totals[3] += float64(state.Momentum[cell].Z)
		totals[4] += float64(state.Energy[cell])
	}

	return totals
}
