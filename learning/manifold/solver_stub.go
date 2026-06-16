//go:build !darwin || !cgo

package manifold

import "fmt"

/*
Solver runs the predictive-coding resonance manifold on Metal (darwin + cgo only).
*/
type Solver struct{}

func NewSolver(arch []int, targetDim int, alpha float64) (*Solver, error) {
	return nil, fmt.Errorf("resonance: Metal manifold solver requires darwin with cgo enabled")
}

func (solver *Solver) Close() {}

func (solver *Solver) SeedWeights(w, r, a, v []float32) error {
	return errUnavailable()
}

func (solver *Solver) ResetState(resetPrecision bool) error {
	return errUnavailable()
}

func (solver *Solver) Settle(input []float64, advanceTemporal bool) error {
	return errUnavailable()
}

func (solver *Solver) Learn(target []float64) error {
	return errUnavailable()
}

func (solver *Solver) Energy() (float64, error) {
	return 0, errUnavailable()
}

func (solver *Solver) ReconstructionError() (float64, error) {
	return 0, errUnavailable()
}

func (solver *Solver) LatentState() ([]float64, error) {
	return nil, errUnavailable()
}

func (solver *Solver) Weights() (w, r, a, v []float32, err error) {
	return nil, nil, nil, nil, errUnavailable()
}

func errUnavailable() error {
	return fmt.Errorf("resonance: Metal manifold solver unavailable on this platform")
}
