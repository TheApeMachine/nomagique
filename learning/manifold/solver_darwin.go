//go:build darwin && cgo

//go:generate go run ./metallibgen

package manifold

/*
#cgo darwin CFLAGS: -x objective-c -fobjc-arc -I${SRCDIR}
#cgo darwin LDFLAGS: -framework Metal -framework Foundation
#include "bridge.h"
#include <stdlib.h>
#include <dispatch/dispatch.h>
*/
import "C"

import (
	_ "embed"
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"unsafe"
)

//go:embed kernels.metallib
var resonanceMetallib []byte

/*
BatchSolver settles N independent predictive-coding resonance manifolds in
lockstep on Metal — one per symbol, each with its own weights. This is the shape
in which the GPU wins: a single symbol's settle is a sequential dependency chain
that can't fill the device, but N symbols advance together so every kernel does
N x the work per GPU roundtrip, amortizing command-buffer latency across the
whole batch.

Each symbol is seeded, fed an input (and optional target) per tick, settled, and
optionally learned — all in batch. Weights/latents/energy are read back per slot.
The per-symbol math is identical to learning.ResonanceManifold (parity-checked).
*/
type BatchSolver struct {
	handle    unsafe.Pointer
	cfg       Config
	arch      []int
	targetDim int
	batch     int

	errBuf             [512]byte
	inputScratch       []float32
	targetScratch      []float32
	batchInputScratch  []float32
	batchTargetScratch []float32
	outcomeLatent      []float32
	outcomeEnergy      []float32
	outcomeSurprise    []float32
}

/*
NewBatchSolver builds a batched manifold for `batch` symbols over the given
architecture and supervised target dimension (0 disables the task head). alpha is
the shared learning pace. Every slot is seeded with the reference seeded-RNG
initialization; call SeedSlot to override a slot with specific weights.
*/
func NewBatchSolver(arch []int, targetDim, batch int, alpha float64) (*BatchSolver, error) {
	if len(arch) < 2 {
		return nil, fmt.Errorf("resonance: architecture must contain at least input and one latent layer")
	}
	if batch <= 0 {
		return nil, fmt.Errorf("resonance: batch size must be positive")
	}

	cfg := AdaptiveConfig(alpha, arch)

	cArch := make([]C.uint32_t, len(arch))
	for i, dim := range arch {
		if dim <= 0 {
			return nil, fmt.Errorf("resonance: layer %d has non-positive dimension %d", i, dim)
		}
		cArch[i] = C.uint32_t(dim)
	}

	if len(resonanceMetallib) == 0 {
		return nil, fmt.Errorf("resonance: embedded kernels.metallib is empty")
	}

	cConfig := cConfigFrom(cfg)
	var errBuf [512]byte

	handle := C.batch_solver_create(
		&cConfig,
		(*C.uint32_t)(unsafe.Pointer(&cArch[0])),
		C.uint32_t(len(arch)),
		C.uint32_t(targetDim),
		C.uint32_t(batch),
		unsafe.Pointer(&resonanceMetallib[0]),
		C.size_t(len(resonanceMetallib)),
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.int(len(errBuf)),
	)

	if handle == nil {
		return nil, fmt.Errorf("resonance: %s", cString(errBuf[:]))
	}

	solver := &BatchSolver{
		handle:          handle,
		cfg:             cfg,
		arch:            append([]int(nil), arch...),
		targetDim:       targetDim,
		batch:           batch,
		outcomeLatent:   make([]float32, batch*arch[len(arch)-1]),
		outcomeEnergy:   make([]float32, batch),
		outcomeSurprise: make([]float32, batch),
	}

	w, r, a, v := InitialWeights(arch, targetDim)
	if err := solver.seedAllSlots(w, r, a, v); err != nil {
		solver.Close()
		return nil, err
	}

	return solver, nil
}

func cConfigFrom(cfg Config) C.ResonanceConfig {
	boolU32 := func(b bool) C.uint32_t {
		if b {
			return 1
		}
		return 0
	}
	return C.ResonanceConfig{
		max_inference_steps:  C.uint32_t(cfg.MaxInferenceSteps),
		min_inference_steps:  C.uint32_t(cfg.MinInferenceSteps),
		lr_state:             C.float(cfg.LrState),
		early_stop_tol:       C.float(cfg.EarlyStopTol),
		early_stop_patience:  C.uint32_t(cfg.EarlyStopPatience),
		monotone_state_steps: boolU32(cfg.MonotoneStateSteps),
		line_search_halvings: C.uint32_t(cfg.LineSearchHalvings),
		lr_generative:        C.float(cfg.LrGenerative),
		lr_temporal:          C.float(cfg.LrTemporal),
		lr_recognition:       C.float(cfg.LrRecognition),
		temporal_weight:      C.float(cfg.TemporalWeight),
		top_down_init_mix:    C.float(cfg.TopDownInitMix),
		use_precision:        boolU32(cfg.UsePrecision),
		precision_beta:       C.float(cfg.PrecisionBeta),
		precision_min:        C.float(cfg.PrecisionMin),
		precision_max:        C.float(cfg.PrecisionMax),
		precision_eps:        C.float(cfg.PrecisionEps),
		latent_decay:         C.float(cfg.LatentDecay),
		sparsity:             C.float(cfg.Sparsity),
		weight_decay:         C.float(cfg.WeightDecay),
		grad_clip:            C.float(cfg.GradClip),
		state_clip:           C.float(cfg.StateClip),
	}
}

/*
InitialWeights returns He/Xavier weights from the same seeded RNG and draw order
as learning.NewResonanceManifold, flat row-major.
*/
func InitialWeights(arch []int, targetDim int) (w, r, a, v []float32) {
	rng := rand.New(rand.NewSource(42))
	wTotal, rTotal, aTotal, vTotal := weightSizes(arch, targetDim)
	w = make([]float32, 0, wTotal)
	r = make([]float32, 0, rTotal)
	a = make([]float32, 0, aTotal)
	if vTotal > 0 {
		v = make([]float32, 0, vTotal)
	}

	numLinks := len(arch) - 1
	for layer := 0; layer < numLinks; layer++ {
		rows, cols := arch[layer], arch[layer+1]
		scaleW := math.Sqrt(2.0 / float64(rows+cols))
		for i := 0; i < rows*cols; i++ {
			w = append(w, float32(rng.NormFloat64()*scaleW))
		}
		scaleR := math.Sqrt(2.0 / float64(rows+cols))
		for i := 0; i < cols*rows; i++ {
			r = append(r, float32(rng.NormFloat64()*scaleR))
		}
	}

	topDim := arch[len(arch)-1]
	scaleA := math.Sqrt(1.0 / float64(topDim))
	for i := 0; i < topDim*topDim; i++ {
		a = append(a, float32(rng.NormFloat64()*scaleA*0.30))
	}

	if targetDim > 0 {
		scaleV := math.Sqrt(2.0 / float64(topDim+targetDim))
		for i := 0; i < targetDim*topDim; i++ {
			v = append(v, float32(rng.NormFloat64()*scaleV))
		}
	}

	return w, r, a, v
}

func weightSizes(arch []int, targetDim int) (wTotal, rTotal, aTotal, vTotal int) {
	for layer := 0; layer < len(arch)-1; layer++ {
		rows, cols := arch[layer], arch[layer+1]
		wTotal += rows * cols
		rTotal += cols * rows
	}
	topDim := arch[len(arch)-1]
	aTotal = topDim * topDim
	if targetDim > 0 {
		vTotal = targetDim * topDim
	}
	return wTotal, rTotal, aTotal, vTotal
}

/*
SeedSlot uploads flat row-major weights for one symbol (W,R concatenated across
links; A the top temporal matrix; V the task head or nil).
*/
func (s *BatchSolver) SeedSlot(slot int, w, r, a, v []float32) error {
	if err := s.validateSlot(slot); err != nil {
		return err
	}

	wP, wL := floatPtr(w)
	rP, rL := floatPtr(r)
	aP, aL := floatPtr(a)
	vP, vL := floatPtr(v)
	err := s.call(func(errBuf []byte) C.int {
		return C.batch_solver_seed_weights(
			s.handle, C.uint32_t(slot),
			wP, C.size_t(wL), rP, C.size_t(rL),
			aP, C.size_t(aL), vP, C.size_t(vL),
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)),
		)
	})
	runtime.KeepAlive(w)
	runtime.KeepAlive(r)
	runtime.KeepAlive(a)
	runtime.KeepAlive(v)
	return err
}

func (s *BatchSolver) seedAllSlots(w, r, a, v []float32) error {
	wP, wL := floatPtr(w)
	rP, rL := floatPtr(r)
	aP, aL := floatPtr(a)
	vP, vL := floatPtr(v)
	err := s.call(func(errBuf []byte) C.int {
		return C.batch_solver_seed_all_weights(
			s.handle,
			wP, C.size_t(wL), rP, C.size_t(rL),
			aP, C.size_t(aL), vP, C.size_t(vL),
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)),
		)
	})
	runtime.KeepAlive(w)
	runtime.KeepAlive(r)
	runtime.KeepAlive(a)
	runtime.KeepAlive(v)
	return err
}

func (s *BatchSolver) Close() {
	if s == nil || s.handle == nil {
		return
	}
	C.batch_solver_destroy(s.handle)
	s.handle = nil
}

func (s *BatchSolver) Batch() int { return s.batch }

func (s *BatchSolver) ResetState(resetPrecision bool) error {
	reset := C.uint32_t(0)
	if resetPrecision {
		reset = 1
	}
	return s.call(func(errBuf []byte) C.int {
		return C.batch_solver_reset_state(s.handle, reset,
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)))
	})
}

/*
SetInput stages symbol `slot`'s input (length arch[0]) and optional target
(length targetDim, or nil) for the next Settle/Learn.
*/
func (s *BatchSolver) SetInput(slot int, input, target []float64) error {
	if err := s.validateSlot(slot); err != nil {
		return err
	}
	if len(input) != s.arch[0] {
		return fmt.Errorf("resonance: input dimension mismatch")
	}
	if len(target) > 0 && (s.targetDim <= 0 || len(target) != s.targetDim) {
		return fmt.Errorf("resonance: target dimension mismatch")
	}
	in := toFloat32Into(s.inputScratch, input)
	s.inputScratch = in

	var (
		targetSlice []float32
		tPtr        *C.float
		tLen        C.uint32_t
	)
	if len(target) == s.targetDim && s.targetDim > 0 {
		targetSlice = toFloat32Into(s.targetScratch, target)
		s.targetScratch = targetSlice
		tPtr = (*C.float)(unsafe.Pointer(&targetSlice[0]))
		tLen = C.uint32_t(len(targetSlice))
	}

	err := s.call(func(errBuf []byte) C.int {
		return C.batch_solver_set_input(
			s.handle, C.uint32_t(slot),
			(*C.float)(unsafe.Pointer(&in[0])), C.uint32_t(len(in)),
			tPtr, tLen,
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)),
		)
	})
	runtime.KeepAlive(in)
	runtime.KeepAlive(targetSlice)
	return err
}

/*
SetInputs stages every slot's input and optional target in one cgo call. Inputs
and targets are slot-major flat slices: slot0, slot1, ... .
*/
func (s *BatchSolver) SetInputs(inputs, targets []float64) error {
	if s == nil || s.handle == nil {
		return fmt.Errorf("resonance: solver is not initialized")
	}
	if len(inputs) != s.batch*s.arch[0] {
		return fmt.Errorf("resonance: batch input dimension mismatch")
	}
	in := toFloat32Into(s.batchInputScratch, inputs)
	s.batchInputScratch = in

	var targetSlice []float32
	if len(targets) > 0 {
		if s.targetDim <= 0 || len(targets) != s.batch*s.targetDim {
			return fmt.Errorf("resonance: batch target dimension mismatch")
		}
		targetSlice = toFloat32Into(s.batchTargetScratch, targets)
		s.batchTargetScratch = targetSlice
	}

	return s.SetInputs32(in, targetSlice)
}

/*
SetInputs32 is the zero-conversion variant of SetInputs for callers that already
maintain float32 staging buffers.
*/
func (s *BatchSolver) SetInputs32(inputs, targets []float32) error {
	if s == nil || s.handle == nil {
		return fmt.Errorf("resonance: solver is not initialized")
	}
	if len(inputs) != s.batch*s.arch[0] {
		return fmt.Errorf("resonance: batch input dimension mismatch")
	}

	var (
		tPtr    *C.float
		tLen    C.uint32_t
		tStride C.uint32_t
	)
	if len(targets) > 0 {
		if s.targetDim <= 0 || len(targets) != s.batch*s.targetDim {
			return fmt.Errorf("resonance: batch target dimension mismatch")
		}
		tPtr = (*C.float)(unsafe.Pointer(&targets[0]))
		tLen = C.uint32_t(len(targets))
		tStride = C.uint32_t(s.targetDim)
	}

	err := s.call(func(errBuf []byte) C.int {
		return C.batch_solver_set_inputs(
			s.handle,
			(*C.float)(unsafe.Pointer(&inputs[0])), C.uint32_t(len(inputs)), C.uint32_t(s.arch[0]),
			tPtr, tLen, tStride,
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)),
		)
	})
	runtime.KeepAlive(inputs)
	runtime.KeepAlive(targets)
	return err
}

func (s *BatchSolver) Settle(advanceTemporal bool) error {
	advance := C.uint32_t(0)
	if advanceTemporal {
		advance = 1
	}
	return s.call(func(errBuf []byte) C.int {
		return C.batch_solver_settle(s.handle, advance,
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)))
	})
}

func (s *BatchSolver) Learn() error {
	return s.call(func(errBuf []byte) C.int {
		return C.batch_solver_learn(s.handle,
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)))
	})
}

/*
ReadOutcomes refreshes post-learn energy and reconstruction for the full batch
in one host transition, then caches latent/energy/surprise in solver scratch.
*/
func (s *BatchSolver) ReadOutcomes() error {
	if s == nil || s.handle == nil {
		return fmt.Errorf("resonance: solver is not initialized")
	}

	topDim := s.arch[len(s.arch)-1]
	expectedLatentLen := s.batch * topDim

	if len(s.outcomeLatent) != expectedLatentLen {
		return fmt.Errorf("resonance: outcome latent buffer length mismatch")
	}

	if len(s.outcomeEnergy) != s.batch || len(s.outcomeSurprise) != s.batch {
		return fmt.Errorf("resonance: outcome scalar buffer length mismatch")
	}

	return s.call(func(errBuf []byte) C.int {
		latentPointer, latentLength := floatPtr(s.outcomeLatent)
		energyPointer, energyLength := floatPtr(s.outcomeEnergy)
		surprisePointer, surpriseLength := floatPtr(s.outcomeSurprise)

		return C.batch_solver_read_outcomes(
			s.handle,
			latentPointer, C.uint32_t(latentLength),
			energyPointer, C.uint32_t(energyLength),
			surprisePointer, C.uint32_t(surpriseLength),
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)),
		)
	})
}

/*
OutcomeSlot returns cached batch outcomes for one slot after ReadOutcomes.
*/
func (s *BatchSolver) OutcomeSlot(slot int) (latent []float64, energy, surprise float64, err error) {
	if err := s.validateSlot(slot); err != nil {
		return nil, 0, 0, err
	}

	topDim := s.arch[len(s.arch)-1]
	start := slot * topDim
	end := start + topDim

	return toFloat64(s.outcomeLatent[start:end]),
		float64(s.outcomeEnergy[slot]),
		float64(s.outcomeSurprise[slot]),
		nil
}

func (s *BatchSolver) TopDimension() int {
	if s == nil || len(s.arch) == 0 {
		return 0
	}

	return s.arch[len(s.arch)-1]
}

func (s *BatchSolver) LatentState(slot int) ([]float64, error) {
	topDim := s.arch[len(s.arch)-1]
	buf := make([]float32, topDim)
	err := s.call(func(errBuf []byte) C.int {
		return C.batch_solver_read_latent(
			s.handle, C.uint32_t(slot),
			(*C.float)(unsafe.Pointer(&buf[0])), C.uint32_t(topDim),
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)),
		)
	})
	if err != nil {
		return nil, err
	}
	return toFloat64(buf), nil
}

func (s *BatchSolver) Energy(slot int) (float64, error) {
	var out C.float
	err := s.call(func(errBuf []byte) C.int {
		return C.batch_solver_read_energy(s.handle, C.uint32_t(slot), &out,
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)))
	})
	return float64(out), err
}

func (s *BatchSolver) ReconstructionError(slot int) (float64, error) {
	var out C.float
	err := s.call(func(errBuf []byte) C.int {
		return C.batch_solver_read_reconstruction(s.handle, C.uint32_t(slot), &out,
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)))
	})
	return float64(out), err
}

func (s *BatchSolver) Weights(slot int) (w, r, a, v []float32, err error) {
	numLinks := len(s.arch) - 1
	wTotal, rTotal := 0, 0
	for l := 0; l < numLinks; l++ {
		wTotal += s.arch[l] * s.arch[l+1]
		rTotal += s.arch[l+1] * s.arch[l]
	}
	top := s.arch[len(s.arch)-1]
	w = make([]float32, wTotal)
	r = make([]float32, rTotal)
	a = make([]float32, top*top)
	if s.targetDim > 0 {
		v = make([]float32, s.targetDim*top)
	}
	wP, wL := floatPtr(w)
	rP, rL := floatPtr(r)
	aP, aL := floatPtr(a)
	vP, vL := floatPtr(v)
	callErr := s.call(func(errBuf []byte) C.int {
		return C.batch_solver_read_weights(
			s.handle, C.uint32_t(slot),
			wP, C.size_t(wL), rP, C.size_t(rL), aP, C.size_t(aL), vP, C.size_t(vL),
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)),
		)
	})
	if callErr != nil {
		return nil, nil, nil, nil, callErr
	}
	return w, r, a, v, nil
}

func (s *BatchSolver) call(run func(errBuf []byte) C.int) error {
	if s == nil || s.handle == nil {
		return fmt.Errorf("resonance: solver is not initialized")
	}
	errBuf := s.errBuf[:]
	for i := range errBuf {
		errBuf[i] = 0
	}
	if run(errBuf) != 0 {
		return fmt.Errorf("resonance: %s", cString(errBuf))
	}
	return nil
}

func (s *BatchSolver) validateSlot(slot int) error {
	if s == nil || s.handle == nil {
		return fmt.Errorf("resonance: solver is not initialized")
	}
	if slot < 0 || slot >= s.batch {
		return fmt.Errorf("resonance: slot out of range")
	}
	return nil
}

func floatPtr(values []float32) (*C.float, int) {
	if len(values) == 0 {
		return nil, 0
	}
	return (*C.float)(unsafe.Pointer(&values[0])), len(values)
}

func toFloat32Into(dst []float32, values []float64) []float32 {
	if cap(dst) < len(values) {
		dst = make([]float32, len(values))
	} else {
		dst = dst[:len(values)]
	}
	for i, v := range values {
		dst[i] = float32(v)
	}
	return dst
}

func toFloat64(values []float32) []float64 {
	out := make([]float64, len(values))
	for i, v := range values {
		out[i] = float64(v)
	}
	return out
}

func cString(buffer []byte) string {
	for i, v := range buffer {
		if v == 0 {
			return string(buffer[:i])
		}
	}
	return string(buffer)
}
