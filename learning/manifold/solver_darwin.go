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

	cConfig := cConfigFrom(cfg)
	errBuf := make([]byte, 512)

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
		return nil, fmt.Errorf("resonance: %s", cString(errBuf))
	}

	solver := &BatchSolver{
		handle:    handle,
		cfg:       cfg,
		arch:      append([]int(nil), arch...),
		targetDim: targetDim,
		batch:     batch,
	}

	w, r, a, v := InitialWeights(arch, targetDim)
	for slot := 0; slot < batch; slot++ {
		if err := solver.SeedSlot(slot, w, r, a, v); err != nil {
			solver.Close()
			return nil, err
		}
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

/*
SeedSlot uploads flat row-major weights for one symbol (W,R concatenated across
links; A the top temporal matrix; V the task head or nil).
*/
func (s *BatchSolver) SeedSlot(slot int, w, r, a, v []float32) error {
	wP, wL := floatPtr(w)
	rP, rL := floatPtr(r)
	aP, aL := floatPtr(a)
	vP, vL := floatPtr(v)
	return s.call(func(errBuf []byte) C.int {
		return C.batch_solver_seed_weights(
			s.handle, C.uint32_t(slot),
			wP, C.size_t(wL), rP, C.size_t(rL),
			aP, C.size_t(aL), vP, C.size_t(vL),
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)),
		)
	})
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
	if len(input) != s.arch[0] {
		return fmt.Errorf("resonance: input dimension mismatch")
	}
	in := toFloat32(input)

	var (
		tPtr *C.float
		tLen C.uint32_t
	)
	if len(target) == s.targetDim && s.targetDim > 0 {
		t := toFloat32(target)
		tPtr = (*C.float)(unsafe.Pointer(&t[0]))
		tLen = C.uint32_t(len(t))
		defer func() { _ = t }()
	}

	return s.call(func(errBuf []byte) C.int {
		return C.batch_solver_set_input(
			s.handle, C.uint32_t(slot),
			(*C.float)(unsafe.Pointer(&in[0])), C.uint32_t(len(in)),
			tPtr, tLen,
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)),
		)
	})
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
	errBuf := make([]byte, 512)
	if run(errBuf) != 0 {
		return fmt.Errorf("resonance: %s", cString(errBuf))
	}
	return nil
}

func floatPtr(values []float32) (*C.float, int) {
	if len(values) == 0 {
		return nil, 0
	}
	return (*C.float)(unsafe.Pointer(&values[0])), len(values)
}

func toFloat32(values []float64) []float32 {
	out := make([]float32, len(values))
	for i, v := range values {
		out[i] = float32(v)
	}
	return out
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
