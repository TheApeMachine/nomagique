//go:build !darwin || !cgo

package manifold

import "fmt"

/*
BatchSolver settles N predictive-coding resonance manifolds on Metal (darwin +
cgo only).
*/
type BatchSolver struct{}

func NewBatchSolver(arch []int, targetDim, batch int, alpha float64) (*BatchSolver, error) {
	return nil, fmt.Errorf("resonance: Metal batch solver requires darwin with cgo enabled")
}

func (s *BatchSolver) Close()     {}
func (s *BatchSolver) Batch() int { return 0 }

func (s *BatchSolver) SeedSlot(slot int, w, r, a, v []float32) error { return errUnavailable() }
func (s *BatchSolver) ResetState(resetPrecision bool) error          { return errUnavailable() }
func (s *BatchSolver) SetInput(slot int, input, target []float64) error {
	return errUnavailable()
}
func (s *BatchSolver) SetInputs(inputs, targets []float64) error {
	return errUnavailable()
}
func (s *BatchSolver) SetInputs32(inputs, targets []float32) error {
	return errUnavailable()
}
func (s *BatchSolver) Settle(advanceTemporal bool) error { return errUnavailable() }
func (s *BatchSolver) Learn() error                      { return errUnavailable() }
func (s *BatchSolver) ReadOutcomes() error               { return errUnavailable() }
func (s *BatchSolver) OutcomeSlot(slot int) ([]float64, float64, float64, error) {
	return nil, 0, 0, errUnavailable()
}
func (s *BatchSolver) TopDimension() int { return 0 }

func (s *BatchSolver) LatentState(slot int) ([]float64, error)       { return nil, errUnavailable() }
func (s *BatchSolver) Energy(slot int) (float64, error)              { return 0, errUnavailable() }
func (s *BatchSolver) ReconstructionError(slot int) (float64, error) { return 0, errUnavailable() }
func (s *BatchSolver) Weights(slot int) (w, r, a, v []float32, err error) {
	return nil, nil, nil, nil, errUnavailable()
}

func errUnavailable() error {
	return fmt.Errorf("resonance: Metal batch solver unavailable on this platform")
}
