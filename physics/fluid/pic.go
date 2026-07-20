package fluid

import (
	"fmt"
	"math"
)

/*
Vector is one three-dimensional simulation vector. The Metal kernels use the
same component order for positions, velocities, and momentum.
*/
type Vector struct {
	X float32
	Y float32
	Z float32
}

/*
Grid defines the periodic Cartesian topology shared by the CPU reference and
the Metal implementation.
*/
type Grid struct {
	X       int     `json:"x"`
	Y       int     `json:"y"`
	Z       int     `json:"z"`
	Spacing float32 `json:"spacing"`
}

/*
Validate rejects grids that cannot define a finite periodic volume.
*/
func (grid Grid) Validate() error {
	if grid.X <= 0 || grid.Y <= 0 || grid.Z <= 0 {
		return fmt.Errorf("fluid: grid dimensions must be positive")
	}

	if grid.Spacing <= 0 || math.IsNaN(float64(grid.Spacing)) || math.IsInf(float64(grid.Spacing), 0) {
		return fmt.Errorf("fluid: grid spacing must be finite and positive")
	}

	return nil
}

/*
Cells returns the number of cells in the grid.
*/
func (grid Grid) Cells() int {
	return grid.X * grid.Y * grid.Z
}

/*
CICStencil contains the eight periodic cell indices and trilinear weights for
one particle. Its weights sum to one apart from binary32 rounding.
*/
type CICStencil struct {
	Indices [8]uint32
	Weights [8]float32
}

/*
CIC computes the periodic cloud-in-cell stencil used by the Sensorium PIC
transfer.
*/
func (grid Grid) CIC(position Vector) (CICStencil, error) {
	if err := grid.Validate(); err != nil {
		return CICStencil{}, err
	}

	domain := Vector{
		X: float32(grid.X) * grid.Spacing,
		Y: float32(grid.Y) * grid.Spacing,
		Z: float32(grid.Z) * grid.Spacing,
	}
	wrapped := Vector{
		X: periodic(position.X, domain.X),
		Y: periodic(position.Y, domain.Y),
		Z: periodic(position.Z, domain.Z),
	}
	cellX := wrapped.X / grid.Spacing
	cellY := wrapped.Y / grid.Spacing
	cellZ := wrapped.Z / grid.Spacing
	baseX := int(math.Floor(float64(cellX)))
	baseY := int(math.Floor(float64(cellY)))
	baseZ := int(math.Floor(float64(cellZ)))
	fractionX := cellX - float32(baseX)
	fractionY := cellY - float32(baseY)
	fractionZ := cellZ - float32(baseZ)

	cellIndicesX := [2]int{modulo(baseX, grid.X), modulo(baseX+1, grid.X)}
	cellIndicesY := [2]int{modulo(baseY, grid.Y), modulo(baseY+1, grid.Y)}
	cellIndicesZ := [2]int{modulo(baseZ, grid.Z), modulo(baseZ+1, grid.Z)}
	weightsX := [2]float32{1 - fractionX, fractionX}
	weightsY := [2]float32{1 - fractionY, fractionY}
	weightsZ := [2]float32{1 - fractionZ, fractionZ}
	corners := [8][3]int{
		{0, 0, 0}, {1, 0, 0}, {0, 1, 0}, {1, 1, 0},
		{0, 0, 1}, {1, 0, 1}, {0, 1, 1}, {1, 1, 1},
	}
	stencil := CICStencil{}

	for index, corner := range corners {
		cellIndex := cellIndicesX[corner[0]]*grid.Y*grid.Z +
			cellIndicesY[corner[1]]*grid.Z + cellIndicesZ[corner[2]]
		stencil.Indices[index] = uint32(cellIndex)
		stencil.Weights[index] = weightsX[corner[0]] *
			weightsY[corner[1]] * weightsZ[corner[2]]
	}

	return stencil, nil
}

/*
ConservedGrid stores the per-volume mass, momentum, and total-energy fields used
by the standalone Torch-reference PIC port. The production Metal domain carries
internal heat separately, as defined by fluid.metal.
*/
type ConservedGrid struct {
	Density  []float32
	Momentum []Vector
	Energy   []float32
}

/*
NewConservedGrid allocates zeroed conserved fields for one topology.
*/
func NewConservedGrid(grid Grid) (*ConservedGrid, error) {
	if err := grid.Validate(); err != nil {
		return nil, err
	}

	return &ConservedGrid{
		Density:  make([]float32, grid.Cells()),
		Momentum: make([]Vector, grid.Cells()),
		Energy:   make([]float32, grid.Cells()),
	}, nil
}

/*
Scatter deposits particle mass, momentum, internal energy, and kinetic energy
into the eight CIC cells as per-volume conserved densities.
*/
func (gridState *ConservedGrid) Scatter(
	grid Grid,
	positions, velocities []Vector,
	masses, internalEnergy []float32,
) error {
	particleCount := len(masses)

	if len(positions) != particleCount || len(velocities) != particleCount ||
		len(internalEnergy) != particleCount {
		return fmt.Errorf("fluid: PIC particle arrays have inconsistent lengths")
	}

	if len(gridState.Density) != grid.Cells() || len(gridState.Momentum) != grid.Cells() ||
		len(gridState.Energy) != grid.Cells() {
		return fmt.Errorf("fluid: PIC output fields do not match the grid")
	}

	inverseVolume := 1 / (grid.Spacing * grid.Spacing * grid.Spacing)

	for particleIndex := range particleCount {
		stencil, err := grid.CIC(positions[particleIndex])

		if err != nil {
			return err
		}

		mass := masses[particleIndex]
		velocity := velocities[particleIndex]
		kinetic := 0.5 * mass *
			(velocity.X*velocity.X + velocity.Y*velocity.Y + velocity.Z*velocity.Z)

		for corner := range stencil.Indices {
			cell := stencil.Indices[corner]
			weight := stencil.Weights[corner] * inverseVolume
			gridState.Density[cell] += mass * weight
			gridState.Momentum[cell].X += mass * velocity.X * weight
			gridState.Momentum[cell].Y += mass * velocity.Y * weight
			gridState.Momentum[cell].Z += mass * velocity.Z * weight
			gridState.Energy[cell] += (internalEnergy[particleIndex] + kinetic) * weight
		}
	}

	return nil
}

/*
GatherScalar samples one scalar cell field at a particle position with the same
CIC weights used during scatter.
*/
func (grid Grid) GatherScalar(position Vector, field []float32) (float32, error) {
	if len(field) != grid.Cells() {
		return 0, fmt.Errorf("fluid: scalar field does not match the grid")
	}

	stencil, err := grid.CIC(position)

	if err != nil {
		return 0, err
	}

	var sampled float32

	for corner, cell := range stencil.Indices {
		sampled += field[cell] * stencil.Weights[corner]
	}

	return sampled, nil
}

/*
GatherVector samples one vector cell field at a particle position with periodic
CIC interpolation.
*/
func (grid Grid) GatherVector(position Vector, field []Vector) (Vector, error) {
	if len(field) != grid.Cells() {
		return Vector{}, fmt.Errorf("fluid: vector field does not match the grid")
	}

	stencil, err := grid.CIC(position)

	if err != nil {
		return Vector{}, err
	}

	var sampled Vector

	for corner, cell := range stencil.Indices {
		weight := stencil.Weights[corner]
		sampled.X += field[cell].X * weight
		sampled.Y += field[cell].Y * weight
		sampled.Z += field[cell].Z * weight
	}

	return sampled, nil
}

func periodic(value, period float32) float32 {
	wrapped := float32(math.Mod(float64(value), float64(period)))

	if wrapped < 0 {
		return wrapped + period
	}

	return wrapped
}

func modulo(value, divisor int) int {
	result := value % divisor

	if result < 0 {
		return result + divisor
	}

	return result
}
