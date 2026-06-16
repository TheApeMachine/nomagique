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
Solver runs the predictive-coding resonance manifold on Metal.

It mirrors learning.ResonanceManifold: a generative/recognition weight stack
settled by free-energy minimization, with adaptive precision and a temporal
prior on the top latent. Weights are initialized with the same seeded RNG as the
gonum reference so the two can be cross-checked within float32 tolerance.
*/
type Solver struct {
	handle    unsafe.Pointer
	cfg       Config
	arch      []int
	targetDim int
}

/*
NewSolver builds a Metal resonance manifold for the given architecture and
supervised target dimension (targetDim == 0 disables the task head). alpha is the
system-wide learning pace fed to AdaptiveConfig.
*/
func NewSolver(arch []int, targetDim int, alpha float64) (*Solver, error) {
	if len(arch) < 2 {
		return nil, fmt.Errorf("resonance: architecture must contain at least input and one latent layer")
	}

	cfg := AdaptiveConfig(alpha, arch)

	cArch := make([]C.uint32_t, len(arch))
	for index, dim := range arch {
		if dim <= 0 {
			return nil, fmt.Errorf("resonance: layer %d has non-positive dimension %d", index, dim)
		}
		cArch[index] = C.uint32_t(dim)
	}

	cConfig := cConfigFrom(cfg)
	errBuf := make([]byte, 512)

	handle := C.manifold_solver_create(
		&cConfig,
		(*C.uint32_t)(unsafe.Pointer(&cArch[0])),
		C.uint32_t(len(arch)),
		C.uint32_t(targetDim),
		unsafe.Pointer(&resonanceMetallib[0]),
		C.size_t(len(resonanceMetallib)),
		(*C.char)(unsafe.Pointer(&errBuf[0])),
		C.int(len(errBuf)),
	)

	if handle == nil {
		return nil, fmt.Errorf("resonance: %s", cString(errBuf))
	}

	solver := &Solver{
		handle:    handle,
		cfg:       cfg,
		arch:      append([]int(nil), arch...),
		targetDim: targetDim,
	}

	if err := solver.seedInitialWeights(); err != nil {
		solver.Close()
		return nil, err
	}

	return solver, nil
}

func cConfigFrom(cfg Config) C.ResonanceConfig {
	boolU32 := func(value bool) C.uint32_t {
		if value {
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
InitialWeights returns the He/Xavier-initialized weight matrices, drawn from the
same seeded RNG and in the same order as the gonum learning.NewResonanceManifold
constructor. They are returned flat (row-major) so callers and parity tests can
compare against the reference directly.
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

func (solver *Solver) seedInitialWeights() error {
	w, r, a, v := InitialWeights(solver.arch, solver.targetDim)
	return solver.SeedWeights(w, r, a, v)
}

/*
SeedWeights uploads flat row-major weight matrices (W and R concatenated across
links, A the top temporal matrix, V the task head or nil). Used both for the
default initialization and to mirror an external reference for parity tests.
*/
func (solver *Solver) SeedWeights(w, r, a, v []float32) error {
	wPtr, wLen := floatPtr(w)
	rPtr, rLen := floatPtr(r)
	aPtr, aLen := floatPtr(a)
	vPtr, vLen := floatPtr(v)

	return solver.call(func(errBuf []byte) C.int {
		return C.manifold_solver_seed_weights(
			solver.handle,
			wPtr, C.size_t(wLen),
			rPtr, C.size_t(rLen),
			aPtr, C.size_t(aLen),
			vPtr, C.size_t(vLen),
			(*C.char)(unsafe.Pointer(&errBuf[0])),
			C.int(len(errBuf)),
		)
	})
}

func (solver *Solver) Close() {
	if solver == nil || solver.handle == nil {
		return
	}

	C.manifold_solver_destroy(solver.handle)
	solver.handle = nil
}

func (solver *Solver) ResetState(resetPrecision bool) error {
	reset := C.uint32_t(0)
	if resetPrecision {
		reset = 1
	}

	return solver.call(func(errBuf []byte) C.int {
		return C.manifold_solver_reset_state(
			solver.handle,
			reset,
			(*C.char)(unsafe.Pointer(&errBuf[0])),
			C.int(len(errBuf)),
		)
	})
}

/*
Settle performs generative inference for one input without supervised
contamination, optionally advancing the temporal prior.
*/
func (solver *Solver) Settle(input []float64, advanceTemporal bool) error {
	if len(input) != solver.arch[0] {
		return fmt.Errorf("resonance: input dimension mismatch")
	}

	in := toFloat32(input)
	advance := C.uint32_t(0)
	if advanceTemporal {
		advance = 1
	}

	return solver.call(func(errBuf []byte) C.int {
		return C.manifold_solver_settle(
			solver.handle,
			(*C.float)(unsafe.Pointer(&in[0])),
			C.uint32_t(len(in)),
			advance,
			(*C.char)(unsafe.Pointer(&errBuf[0])),
			C.int(len(errBuf)),
		)
	})
}

/*
Learn applies weight updates for the current settled state. Pass nil (or a
mismatched length) to skip the supervised task head.
*/
func (solver *Solver) Learn(target []float64) error {
	var t []float32
	if len(target) == solver.targetDim && solver.targetDim > 0 {
		t = toFloat32(target)
	}

	ptr, n := floatPtr(t)

	return solver.call(func(errBuf []byte) C.int {
		return C.manifold_solver_learn(
			solver.handle,
			ptr,
			C.uint32_t(n),
			(*C.char)(unsafe.Pointer(&errBuf[0])),
			C.int(len(errBuf)),
		)
	})
}

func (solver *Solver) Energy() (float64, error) {
	var out C.float
	err := solver.call(func(errBuf []byte) C.int {
		return C.manifold_solver_energy(
			solver.handle, &out,
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)),
		)
	})
	return float64(out), err
}

func (solver *Solver) ReconstructionError() (float64, error) {
	var out C.float
	err := solver.call(func(errBuf []byte) C.int {
		return C.manifold_solver_reconstruction_error(
			solver.handle, &out,
			(*C.char)(unsafe.Pointer(&errBuf[0])), C.int(len(errBuf)),
		)
	})
	return float64(out), err
}

func (solver *Solver) LatentState() ([]float64, error) {
	topDim := solver.arch[len(solver.arch)-1]
	buffer := make([]float32, topDim)

	err := solver.call(func(errBuf []byte) C.int {
		return C.manifold_solver_read_latent(
			solver.handle,
			(*C.float)(unsafe.Pointer(&buffer[0])),
			C.uint32_t(topDim),
			(*C.char)(unsafe.Pointer(&errBuf[0])),
			C.int(len(errBuf)),
		)
	})

	if err != nil {
		return nil, err
	}

	return toFloat64(buffer), nil
}

/*
Weights reads back the current weight matrices using the same flat layout as
SeedWeights.
*/
func (solver *Solver) Weights() (w, r, a, v []float32, err error) {
	numLinks := len(solver.arch) - 1
	wTotal, rTotal := 0, 0
	for layer := 0; layer < numLinks; layer++ {
		wTotal += solver.arch[layer] * solver.arch[layer+1]
		rTotal += solver.arch[layer+1] * solver.arch[layer]
	}
	topDim := solver.arch[len(solver.arch)-1]
	aTotal := topDim * topDim
	vTotal := 0
	if solver.targetDim > 0 {
		vTotal = solver.targetDim * topDim
	}

	w = make([]float32, wTotal)
	r = make([]float32, rTotal)
	a = make([]float32, aTotal)
	if vTotal > 0 {
		v = make([]float32, vTotal)
	}

	wPtr, wLen := floatPtr(w)
	rPtr, rLen := floatPtr(r)
	aPtr, aLen := floatPtr(a)
	vPtr, vLen := floatPtr(v)

	callErr := solver.call(func(errBuf []byte) C.int {
		return C.manifold_solver_read_weights(
			solver.handle,
			wPtr, C.size_t(wLen),
			rPtr, C.size_t(rLen),
			aPtr, C.size_t(aLen),
			vPtr, C.size_t(vLen),
			(*C.char)(unsafe.Pointer(&errBuf[0])),
			C.int(len(errBuf)),
		)
	})

	if callErr != nil {
		return nil, nil, nil, nil, callErr
	}

	return w, r, a, v, nil
}

func (solver *Solver) call(run func(errBuf []byte) C.int) error {
	if solver == nil || solver.handle == nil {
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
	for index, value := range values {
		out[index] = float32(value)
	}
	return out
}

func toFloat64(values []float32) []float64 {
	out := make([]float64, len(values))
	for index, value := range values {
		out[index] = float64(value)
	}
	return out
}

func cString(buffer []byte) string {
	for index, value := range buffer {
		if value == 0 {
			return string(buffer[:index])
		}
	}
	return string(buffer)
}
