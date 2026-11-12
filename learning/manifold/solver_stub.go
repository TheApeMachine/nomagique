//go:build !darwin || !cgo

package manifold

import "fmt"

/*
BatchSolver settles N predictive-coding resonance manifolds on Metal (darwin +
cgo only).
*/
type BatchSolver struct{}

/*
NewBatchSolver fails fast on platforms without Metal so CI and Linux builds
link the same API as darwin without silent no-ops.
*/
func NewBatchSolver(arch []int, targetDim, batch int, alpha float64) (*BatchSolver, error) {
	return nil, fmt.Errorf("resonance: Metal batch solver requires darwin with cgo enabled")
}

/*
Close releases no resources on the stub — present for API symmetry with Metal.
*/
func (s *BatchSolver) Close() {}

/*
Batch returns zero on stub builds so callers detect disabled batching.
*/
func (s *BatchSolver) Batch() int { return 0 }

/*
SeedSlot is unavailable off Metal — weights must stay on the CPU manifold path.
*/
func (s *BatchSolver) SeedSlot(slot int, w, r, a, v []float32) error { return errUnavailable() }

/*
ResetState is unavailable off Metal.
*/
func (s *BatchSolver) ResetState(resetPrecision bool) error { return errUnavailable() }

/*
SetInput is unavailable off Metal.
*/
func (s *BatchSolver) SetInput(slot int, input, target []float64) error {
	return errUnavailable()
}

/*
SetInputs is unavailable off Metal.
*/
func (s *BatchSolver) SetInputs(inputs, targets []float64) error {
	return errUnavailable()
}

/*
SetInputs32 is unavailable off Metal.
*/
func (s *BatchSolver) SetInputs32(inputs, targets []float32) error {
	return errUnavailable()
}

/*
Settle is unavailable off Metal.
*/
func (s *BatchSolver) Settle(advanceTemporal bool) error { return errUnavailable() }

/*
Learn is unavailable off Metal.
*/
func (s *BatchSolver) Learn() error { return errUnavailable() }

/*
ReadOutcomes is unavailable off Metal.
*/
func (s *BatchSolver) ReadOutcomes() error { return errUnavailable() }

/*
OutcomeSlot is unavailable off Metal.
*/
func (s *BatchSolver) OutcomeSlot(slot int) ([]float64, float64, float64, error) {
	return nil, 0, 0, errUnavailable()
}

/*
TopDimension is unavailable off Metal.
*/
func (s *BatchSolver) TopDimension() int { return 0 }

/*
TargetDim is unavailable off Metal.
*/
func (s *BatchSolver) TargetDim() int { return 0 }

/*
TaskPrediction is unavailable off Metal.
*/
func (s *BatchSolver) TaskPrediction(slot int, latent []float64) ([]float64, error) {
	return nil, errUnavailable()
}

/*
LatentState is unavailable off Metal.
*/
func (s *BatchSolver) LatentState(slot int) ([]float64, error) { return nil, errUnavailable() }

/*
Energy is unavailable off Metal.
*/
func (s *BatchSolver) Energy(slot int) (float64, error) { return 0, errUnavailable() }

/*
ReconstructionError is unavailable off Metal.
*/
func (s *BatchSolver) ReconstructionError(slot int) (float64, error) { return 0, errUnavailable() }

/*
Weights is unavailable off Metal.
*/
func (s *BatchSolver) Weights(slot int) (w, r, a, v []float32, err error) {
	return nil, nil, nil, nil, errUnavailable()
}

/*
errUnavailable centralizes stub errors so parity tests see one consistent refusal.
*/
func errUnavailable() error {
	return fmt.Errorf("resonance: Metal batch solver unavailable on this platform")
}
