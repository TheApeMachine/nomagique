package fluid

import "math"

/*
conservedCell packs density, momentum, and total energy for a flux operation.
*/
type conservedCell [5]float32

/*
project enforces the density and internal-energy floors of the CPU reference.
*/
func (gas *Gas) project(state *GasState) *GasState {
	minimumEnergy := gas.numerics.PressureMinimum / (gas.material.Gamma - 1)

	for cell := range state.Density {
		state.Density[cell] = max(state.Density[cell], gas.numerics.DensityMinimum)
		momentum := state.Momentum[cell]
		kinetic := 0.5 * (momentum.X*momentum.X + momentum.Y*momentum.Y +
			momentum.Z*momentum.Z) / state.Density[cell]
		internal := max(state.Energy[cell]-kinetic, minimumEnergy)
		state.Energy[cell] = internal + kinetic
	}

	return state
}

/*
primitive converts one total-energy cell to density, velocity, and pressure.
*/
func (gas *Gas) primitive(state *GasState, cell int) (float32, Vector, float32) {
	density := max(state.Density[cell], gas.numerics.DensityMinimum)
	momentum := state.Momentum[cell]
	velocity := Vector{
		X: momentum.X / density,
		Y: momentum.Y / density,
		Z: momentum.Z / density,
	}
	kinetic := 0.5 * (momentum.X*momentum.X + momentum.Y*momentum.Y +
		momentum.Z*momentum.Z) / density
	pressure := max(
		(gas.material.Gamma-1)*(state.Energy[cell]-kinetic),
		gas.numerics.PressureMinimum,
	)

	return density, velocity, pressure
}

/*
rhs evaluates inviscid flux divergence, transport, and gravity sources.
*/
func (gas *Gas) rhs(state *GasState) *GasState {
	state = gas.project(state.clone())
	fluxes := [3][]conservedCell{
		make([]conservedCell, gas.grid.Cells()),
		make([]conservedCell, gas.grid.Cells()),
		make([]conservedCell, gas.grid.Cells()),
	}

	for direction := range fluxes {
		for cell := range state.Density {
			left := gas.neighbor(cell, direction, -1)
			fluxes[direction][cell] = gas.interfaceFlux(state, left, cell, direction)
		}
	}

	rate := &GasState{
		Density:  make([]float32, gas.grid.Cells()),
		Momentum: make([]Vector, gas.grid.Cells()),
		Energy:   make([]float32, gas.grid.Cells()),
	}
	inverseSpacing := 1 / gas.grid.Spacing

	for cell := range state.Density {
		var divergence conservedCell

		for direction := range fluxes {
			forward := gas.neighbor(cell, direction, 1)

			for component := range divergence {
				divergence[component] += (fluxes[direction][forward][component] -
					fluxes[direction][cell][component]) * inverseSpacing
			}
		}

		rate.Density[cell] = -divergence[0]
		rate.Momentum[cell] = Vector{
			X: -divergence[1],
			Y: -divergence[2],
			Z: -divergence[3],
		}
		rate.Energy[cell] = -divergence[4]
	}

	gas.addTransport(state, rate)
	gas.addGravity(state, rate)
	return rate
}

/*
interfaceFlux evaluates the local Lax-Friedrichs flux at one cell face.
*/
func (gas *Gas) interfaceFlux(
	state *GasState,
	left, right, direction int,
) conservedCell {
	leftDensity, leftVelocity, leftPressure := gas.primitive(state, left)
	rightDensity, rightVelocity, rightPressure := gas.primitive(state, right)
	leftConserved := gas.conserved(state, left, leftDensity)
	rightConserved := gas.conserved(state, right, rightDensity)
	leftFlux := gas.inviscid(state, left, leftVelocity, leftPressure, direction)
	rightFlux := gas.inviscid(state, right, rightVelocity, rightPressure, direction)
	leftSound := float32(math.Sqrt(float64(
		gas.material.Gamma * leftPressure / leftDensity,
	)))
	rightSound := float32(math.Sqrt(float64(
		gas.material.Gamma * rightPressure / rightDensity,
	)))
	maximumSpeed := max(vectorNorm(leftVelocity)+leftSound, vectorNorm(rightVelocity)+rightSound)
	var flux conservedCell

	for component := range flux {
		flux[component] = 0.5*(leftFlux[component]+rightFlux[component]) -
			0.5*maximumSpeed*(rightConserved[component]-leftConserved[component])
	}

	return flux
}

/*
conserved packs one cell in density, three-momentum, total-energy order.
*/
func (gas *Gas) conserved(state *GasState, cell int, density float32) conservedCell {
	momentum := state.Momentum[cell]

	return conservedCell{
		density,
		momentum.X,
		momentum.Y,
		momentum.Z,
		state.Energy[cell],
	}
}

/*
inviscid returns the ideal-gas conserved flux along one Cartesian direction.
*/
func (gas *Gas) inviscid(
	state *GasState,
	cell int,
	velocity Vector,
	pressure float32,
	direction int,
) conservedCell {
	momentum := state.Momentum[cell]
	components := [3]float32{velocity.X, velocity.Y, velocity.Z}
	axisVelocity := components[direction]
	flux := conservedCell{
		momentumComponent(momentum, direction),
		momentum.X * axisVelocity,
		momentum.Y * axisVelocity,
		momentum.Z * axisVelocity,
		(state.Energy[cell] + pressure) * axisVelocity,
	}
	flux[direction+1] += pressure
	return flux
}

/*
addTransport adds constant-coefficient viscous work and thermal conduction.
*/
func (gas *Gas) addTransport(state, rate *GasState) {
	velocity := [3][]float32{
		make([]float32, gas.grid.Cells()),
		make([]float32, gas.grid.Cells()),
		make([]float32, gas.grid.Cells()),
	}
	temperature := make([]float32, gas.grid.Cells())

	for cell := range state.Density {
		density, current, pressure := gas.primitive(state, cell)
		velocity[0][cell] = current.X
		velocity[1][cell] = current.Y
		velocity[2][cell] = current.Z
		temperature[cell] = pressure / (density * gas.material.SpecificGasConstant)
	}

	gradient := [3][3][]float32{}

	for component := range velocity {
		for direction := range velocity {
			gradient[component][direction] = gas.centralDifference(velocity[component], direction)
		}
	}

	divergence := make([]float32, gas.grid.Cells())

	for cell := range divergence {
		divergence[cell] = gradient[0][0][cell] +
			gradient[1][1][cell] + gradient[2][2][cell]
	}

	gradientDivergence := [3][]float32{
		gas.centralDifference(divergence, 0),
		gas.centralDifference(divergence, 1),
		gas.centralDifference(divergence, 2),
	}
	laplacianVelocity := [3][]float32{
		gas.laplacian(velocity[0]),
		gas.laplacian(velocity[1]),
		gas.laplacian(velocity[2]),
	}
	viscosity := gas.material.DynamicViscosity
	bulk := -(2.0 / 3.0) * viscosity
	work := [3][]float32{
		make([]float32, gas.grid.Cells()),
		make([]float32, gas.grid.Cells()),
		make([]float32, gas.grid.Cells()),
	}

	for cell := range state.Density {
		rate.Momentum[cell].X += viscosity*laplacianVelocity[0][cell] +
			(viscosity+bulk)*gradientDivergence[0][cell]
		rate.Momentum[cell].Y += viscosity*laplacianVelocity[1][cell] +
			(viscosity+bulk)*gradientDivergence[1][cell]
		rate.Momentum[cell].Z += viscosity*laplacianVelocity[2][cell] +
			(viscosity+bulk)*gradientDivergence[2][cell]

		tauXX := 2*viscosity*gradient[0][0][cell] + bulk*divergence[cell]
		tauYY := 2*viscosity*gradient[1][1][cell] + bulk*divergence[cell]
		tauZZ := 2*viscosity*gradient[2][2][cell] + bulk*divergence[cell]
		tauXY := viscosity * (gradient[0][1][cell] + gradient[1][0][cell])
		tauXZ := viscosity * (gradient[0][2][cell] + gradient[2][0][cell])
		tauYZ := viscosity * (gradient[1][2][cell] + gradient[2][1][cell])
		velocityX := velocity[0][cell]
		velocityY := velocity[1][cell]
		velocityZ := velocity[2][cell]
		work[0][cell] = tauXX*velocityX + tauXY*velocityY + tauXZ*velocityZ
		work[1][cell] = tauXY*velocityX + tauYY*velocityY + tauYZ*velocityZ
		work[2][cell] = tauXZ*velocityX + tauYZ*velocityY + tauZZ*velocityZ
	}

	workX := gas.centralDifference(work[0], 0)
	workY := gas.centralDifference(work[1], 1)
	workZ := gas.centralDifference(work[2], 2)
	laplacianTemperature := gas.laplacian(temperature)

	for cell := range state.Density {
		rate.Energy[cell] += workX[cell] + workY[cell] + workZ[cell] +
			gas.material.ThermalConductivity*laplacianTemperature[cell]
	}
}

/*
addGravity applies acceleration to momentum and its corresponding power to
total energy.
*/
func (gas *Gas) addGravity(state, rate *GasState) {
	if len(gas.gravity) == 0 {
		return
	}

	for cell, acceleration := range gas.gravity {
		density, velocity, _ := gas.primitive(state, cell)
		force := Vector{
			X: density * acceleration.X,
			Y: density * acceleration.Y,
			Z: density * acceleration.Z,
		}
		rate.Momentum[cell].X += force.X
		rate.Momentum[cell].Y += force.Y
		rate.Momentum[cell].Z += force.Z
		rate.Energy[cell] += force.X*velocity.X + force.Y*velocity.Y + force.Z*velocity.Z
	}
}

/*
centralDifference evaluates the second-order periodic central derivative.
*/
func (gas *Gas) centralDifference(field []float32, direction int) []float32 {
	derivative := make([]float32, len(field))
	scale := 0.5 / gas.grid.Spacing

	for cell := range field {
		forward := gas.neighbor(cell, direction, 1)
		backward := gas.neighbor(cell, direction, -1)
		derivative[cell] = (field[forward] - field[backward]) * scale
	}

	return derivative
}

/*
laplacian evaluates the periodic three-dimensional seven-point stencil.
*/
func (gas *Gas) laplacian(field []float32) []float32 {
	result := make([]float32, len(field))
	inverseSpacingSquared := 1 / (gas.grid.Spacing * gas.grid.Spacing)

	for cell := range field {
		value := -6 * field[cell]

		for direction := range 3 {
			value += field[gas.neighbor(cell, direction, -1)]
			value += field[gas.neighbor(cell, direction, 1)]
		}

		result[cell] = value * inverseSpacingSquared
	}

	return result
}

/*
neighbor resolves a periodic Cartesian neighbor from a flattened cell index.
*/
func (gas *Gas) neighbor(cell, direction, offset int) int {
	plane := gas.grid.Y * gas.grid.Z
	x := cell / plane
	y := (cell / gas.grid.Z) % gas.grid.Y
	z := cell % gas.grid.Z
	coordinates := [3]int{x, y, z}
	dimensions := [3]int{gas.grid.X, gas.grid.Y, gas.grid.Z}
	coordinates[direction] = modulo(coordinates[direction]+offset, dimensions[direction])

	return coordinates[0]*plane + coordinates[1]*gas.grid.Z + coordinates[2]
}

func momentumComponent(momentum Vector, direction int) float32 {
	return [3]float32{momentum.X, momentum.Y, momentum.Z}[direction]
}

func vectorNorm(vector Vector) float32 {
	return float32(math.Sqrt(float64(
		vector.X*vector.X + vector.Y*vector.Y + vector.Z*vector.Z,
	)))
}
