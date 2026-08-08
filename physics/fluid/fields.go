package fluid

import "fmt"

/*
Fields is one coherent post-step copy of the complete Eulerian gas state and
spatial complex wave field. Scalar arrays use the domain's X-major, Z-fastest
cell order; Momentum stores XYZ triples in the same cell order.
*/
type Fields struct {
	Grid           Grid
	Density        []float32
	Momentum       []float32
	InternalEnergy []float32
	WaveReal       []float32
	WaveImaginary  []float32
}

/*
Validate verifies that every field has exactly the shape declared by Grid.
*/
func (fields Fields) Validate() error {
	if err := fields.Grid.Validate(); err != nil {
		return err
	}

	cells := fields.Grid.X * fields.Grid.Y * fields.Grid.Z

	if len(fields.Density) != cells || len(fields.InternalEnergy) != cells ||
		len(fields.WaveReal) != cells || len(fields.WaveImaginary) != cells {
		return fmt.Errorf("fluid: scalar field shape does not match %d-cell lattice", cells)
	}

	if len(fields.Momentum) != cells*3 {
		return fmt.Errorf("fluid: momentum field shape does not match %d-cell lattice", cells)
	}

	return nil
}
