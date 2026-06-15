package learning

import (
	"errors"
	"math"
	"math/rand"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/mat"
)

type ResonanceConfigV2 struct {
	MaxInferenceSteps  int
	MinInferenceSteps  int
	LrState            float64
	EarlyStopTol       float64
	EarlyStopPatience  int
	MonotoneStateSteps bool
	LineSearchHalvings int

	LrGenerative  float64
	LrTemporal    float64
	LrRecognition float64

	TemporalWeight float64
	TopDownInitMix float64

	UsePrecision  bool
	PrecisionBeta float64
	PrecisionMin  float64
	PrecisionMax  float64
	PrecisionEps  float64

	LatentDecay float64
	Sparsity    float64
	WeightDecay float64
	GradClip    float64
	StateClip   float64
}

/*
AdaptiveResonanceConfig derives every single hyperparameter dynamically
from the system-wide learning pace (alpha) and the physical depth of the network.
*/
func AdaptiveResonanceConfig(alpha float64, arch []int) ResonanceConfigV2 {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.010
	}
	depth := len(arch)

	return ResonanceConfigV2{
		MaxInferenceSteps:  depth * 8,
		MinInferenceSteps:  depth * 2,
		LrState:            alpha * 10.0,
		EarlyStopTol:       1e-5,
		EarlyStopPatience:  3,
		MonotoneStateSteps: true,
		LineSearchHalvings: 3,

		LrGenerative:  alpha * 1.0,
		LrTemporal:    alpha * 2.0,
		LrRecognition: alpha * 0.6,

		TemporalWeight: 0.45,
		TopDownInitMix: 0.55,

		UsePrecision:  true,
		PrecisionBeta: alpha,
		PrecisionMin:  0.10,
		PrecisionMax:  5.0,
		PrecisionEps:  1e-4,

		LatentDecay: alpha * 1e-1,
		Sparsity:    1e-4,
		WeightDecay: alpha * 1e-3,
		GradClip:    1.0,
		StateClip:   3.0,
	}
}

type ResonanceManifoldV2 struct {
	artifact          *datura.Artifact
	cfg               ResonanceConfigV2
	arch              []int
	targetDim         int
	W                 []*mat.Dense // Generative weights W[l]: z[l+1] -> z[l]
	R                 []*mat.Dense // Recognition weights R[l]: z[l] -> z[l+1]
	A                 *mat.Dense   // Temporal prior A: z_L(t-1) -> z_L(t)
	V                 *mat.Dense   // Supervised task weights V: z_L -> y
	z                 []*mat.Dense // Latent states z[l]
	prevTop           *mat.Dense   // z_L(t-1)
	errorVar          []*mat.Dense // Variance tracking for error channels
	precision         []*mat.Dense // Precision (inverse variance) weightings
	temporalVar       *mat.Dense
	temporalPrecision *mat.Dense
	taskVar           *mat.Dense // Variance tracking for the task error
	taskPrecision     *mat.Dense // Precision weighting for the task
	output            float64
}

func NewResonanceManifoldV2(arch []int, targetDim int, alpha float64) (*ResonanceManifoldV2, error) {
	if len(arch) < 2 {
		return nil, errors.New("resonance: architecture must contain at least input and one latent layer")
	}

	cfg := AdaptiveResonanceConfig(alpha, arch)
	rng := rand.New(rand.NewSource(42))
	numLinks := len(arch) - 1

	w := make([]*mat.Dense, numLinks)
	rRec := make([]*mat.Dense, numLinks)
	errorVar := make([]*mat.Dense, numLinks)
	precision := make([]*mat.Dense, numLinks)

	for l := 0; l < numLinks; l++ {
		r, c := arch[l], arch[l+1]

		scaleW := math.Sqrt(2.0 / float64(r+c))
		dataW := make([]float64, r*c)
		for i := range dataW {
			dataW[i] = rng.NormFloat64() * scaleW
		}
		w[l] = mat.NewDense(r, c, dataW)

		scaleR := math.Sqrt(2.0 / float64(r+c))
		dataR := make([]float64, r*c)
		for i := range dataR {
			dataR[i] = rng.NormFloat64() * scaleR
		}
		rRec[l] = mat.NewDense(c, r, dataR)

		errorVar[l] = mat.NewDense(r, 1, nil)
		precision[l] = mat.NewDense(r, 1, nil)
		for i := 0; i < r; i++ {
			errorVar[l].Set(i, 0, 1.0)
			precision[l].Set(i, 0, 1.0)
		}
	}

	topDim := arch[len(arch)-1]
	scaleA := math.Sqrt(1.0 / float64(topDim))
	dataA := make([]float64, topDim*topDim)
	for i := range dataA {
		dataA[i] = rng.NormFloat64() * scaleA * 0.30
	}
	a := mat.NewDense(topDim, topDim, dataA)

	var vWeights *mat.Dense
	var taskVar *mat.Dense
	var taskPrecision *mat.Dense

	if targetDim > 0 {
		scaleV := math.Sqrt(2.0 / float64(topDim+targetDim))
		dataV := make([]float64, targetDim*topDim)
		for i := range dataV {
			dataV[i] = rng.NormFloat64() * scaleV
		}
		vWeights = mat.NewDense(targetDim, topDim, dataV)

		taskVar = mat.NewDense(targetDim, 1, nil)
		taskPrecision = mat.NewDense(targetDim, 1, nil)
		for i := 0; i < targetDim; i++ {
			taskVar.Set(i, 0, 1.0)
			taskPrecision.Set(i, 0, 1.0)
		}
	}

	z := make([]*mat.Dense, len(arch))
	for l, dim := range arch {
		z[l] = mat.NewDense(dim, 1, nil)
	}

	temporalVar := mat.NewDense(topDim, 1, nil)
	temporalPrecision := mat.NewDense(topDim, 1, nil)
	for i := 0; i < topDim; i++ {
		temporalVar.Set(i, 0, 1.0)
		temporalPrecision.Set(i, 0, 1.0)
	}

	return &ResonanceManifoldV2{
		artifact:          datura.Acquire("resonance", datura.Artifact_Type_json),
		cfg:               cfg,
		arch:              arch,
		targetDim:         targetDim,
		W:                 w,
		R:                 rRec,
		A:                 a,
		V:                 vWeights,
		z:                 z,
		errorVar:          errorVar,
		precision:         precision,
		temporalVar:       temporalVar,
		temporalPrecision: temporalPrecision,
		taskVar:           taskVar,
		taskPrecision:     taskPrecision,
	}, nil
}

func (rm *ResonanceManifoldV2) Reset() error {
	rm.ResetState(false)
	return nil
}

func (rm *ResonanceManifoldV2) ResetState(resetPrecision bool) {
	for _, z := range rm.z {
		r, _ := z.Dims()
		for i := 0; i < r; i++ {
			z.Set(i, 0, 0.0)
		}
	}
	rm.prevTop = nil

	if resetPrecision {
		for l := 0; l < len(rm.W); l++ {
			r, _ := rm.errorVar[l].Dims()
			for i := 0; i < r; i++ {
				rm.errorVar[l].Set(i, 0, 1.0)
				rm.precision[l].Set(i, 0, 1.0)
			}
		}
		topDim := rm.arch[len(rm.arch)-1]
		for i := 0; i < topDim; i++ {
			rm.temporalVar.Set(i, 0, 1.0)
			rm.temporalPrecision.Set(i, 0, 1.0)
		}
		if rm.targetDim > 0 {
			for i := 0; i < rm.targetDim; i++ {
				rm.taskVar.Set(i, 0, 1.0)
				rm.taskPrecision.Set(i, 0, 1.0)
			}
		}
	}
}

func (rm *ResonanceManifoldV2) Write(p []byte) (int, error) {
	return rm.artifact.Write(p)
}

func (rm *ResonanceManifoldV2) Read(p []byte) (int, error) {
	values := float64Batch(rm.artifact)

	if len(values) >= rm.arch[0] {
		input := values[:rm.arch[0]]
		var target []float64

		if rm.targetDim > 0 && len(values) >= rm.arch[0]+rm.targetDim {
			target = values[rm.arch[0] : rm.arch[0]+rm.targetDim]
		}

		recon := rm.SettleFromBatch(input, target)
		putFloat64Payload(&rm.artifact, "resonance", recon)
	}

	return rm.artifact.Read(p)
}

func (rm *ResonanceManifoldV2) Close() error {
	return nil
}

func (rm *ResonanceManifoldV2) SettleFromBatch(x []float64, y []float64) float64 {
	if len(x) != rm.arch[0] {
		padded := make([]float64, rm.arch[0])
		copy(padded, x)
		x = padded
	}

	_ = rm.Settle(x, y)
	rm.Learn(y)

	recon := rm.ReconstructionError()
	rm.output = recon

	return rm.output
}

func (rm *ResonanceManifoldV2) ReconstructionOutput() float64 {
	return rm.output
}

func (rm *ResonanceManifoldV2) Settle(x []float64, y []float64) error {
	xCol := mat.NewDense(rm.arch[0], 1, x)
	var yCol *mat.Dense
	if len(y) == rm.targetDim && rm.targetDim > 0 {
		yCol = mat.NewDense(rm.targetDim, 1, y)
	}

	rm.initializeLatents(xCol)

	trace := []float64{rm.Energy(yCol)}

	for step := 0; step < rm.cfg.MaxInferenceSteps; step++ {
		predictions, errors := rm.predictAdjacentLayers()
		grads := rm.stateGradients(predictions, errors, yCol)

		oldStates := copyStates(rm.z)
		oldEnergy := trace[len(trace)-1]
		accepted := false
		stepSize := rm.cfg.LrState

		halvings := 0
		if rm.cfg.MonotoneStateSteps {
			halvings = rm.cfg.LineSearchHalvings
		}

		for h := 0; h <= halvings; h++ {
			candidate := rm.applyStateUpdate(oldStates, grads, stepSize)
			rm.z = candidate
			rm.z[0] = xCol
			newEnergy := rm.Energy(yCol)

			if !rm.cfg.MonotoneStateSteps || newEnergy <= oldEnergy+1e-12 {
				accepted = true
				break
			}
			stepSize *= 0.5
		}

		if !accepted {
			rm.z = oldStates
			rm.z[0] = xCol
			trace = append(trace, oldEnergy)
		} else {
			trace = append(trace, rm.Energy(yCol))
		}

		deltaE := math.Abs(trace[len(trace)-2] - trace[len(trace)-1])
		relativeDelta := deltaE / (math.Abs(trace[len(trace)-2]) + 1e-12)

		if step+1 >= rm.cfg.MinInferenceSteps && relativeDelta < rm.cfg.EarlyStopTol {
			break
		}
	}

	return nil
}

func (rm *ResonanceManifoldV2) Learn(y []float64) {
	predictions, errors := rm.predictAdjacentLayers()
	topIdx := len(rm.z) - 1

	var yCol *mat.Dense
	if len(y) == rm.targetDim && rm.targetDim > 0 {
		yCol = mat.NewDense(rm.targetDim, 1, y)
	}

	// 1. Update Generative Weights W
	for l, W := range rm.W {
		localSignal := mat.NewDense(rm.arch[l], 1, nil)
		localSignal.Apply(func(r, c int, v float64) float64 {
			return 1.0 - v*v
		}, predictions[l])

		prec := rm.precisionFor(l)
		localSignal.MulElem(localSignal, errors[l])
		localSignal.MulElem(localSignal, prec)

		update := mat.NewDense(rm.arch[l], rm.arch[l+1], nil)
		update.Outer(1.0, localSignal.ColView(0), rm.z[l+1].ColView(0))

		norm := mat.Norm(update, 2)
		if norm > rm.cfg.GradClip {
			update.Scale(rm.cfg.GradClip/(norm+1e-12), update)
		}

		update.Scale(rm.cfg.LrGenerative, update)
		W.Add(W, update)

		if rm.cfg.WeightDecay > 0 {
			W.Scale(1.0-rm.cfg.LrGenerative*rm.cfg.WeightDecay, W)
		}
	}

	// 2. Update Recognition Weights R
	for l, R := range rm.R {
		proposal := mat.NewDense(rm.arch[l+1], 1, nil)
		proposal.Mul(R, rm.z[l])
		proposal.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, proposal)

		recError := mat.NewDense(rm.arch[l+1], 1, nil)
		recError.Sub(rm.z[l+1], proposal)

		recSignal := mat.NewDense(rm.arch[l+1], 1, nil)
		recSignal.Apply(func(r, c int, v float64) float64 { return 1.0 - v*v }, proposal)
		recSignal.MulElem(recSignal, recError)

		update := mat.NewDense(rm.arch[l+1], rm.arch[l], nil)
		update.Outer(1.0, recSignal.ColView(0), rm.z[l].ColView(0))

		norm := mat.Norm(update, 2)
		if norm > rm.cfg.GradClip {
			update.Scale(rm.cfg.GradClip/(norm+1e-12), update)
		}

		update.Scale(rm.cfg.LrRecognition, update)
		R.Add(R, update)

		if rm.cfg.WeightDecay > 0 {
			R.Scale(1.0-rm.cfg.LrRecognition*rm.cfg.WeightDecay, R)
		}
	}

	// 3. Update Task-Gated Supervised Weights V
	if yCol != nil && rm.V != nil {
		taskPred := mat.NewDense(rm.targetDim, 1, nil)
		taskPred.Mul(rm.V, rm.z[topIdx])
		taskPred.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, taskPred)

		taskError := mat.NewDense(rm.targetDim, 1, nil)
		taskError.Sub(yCol, taskPred)

		taskSignal := mat.NewDense(rm.targetDim, 1, nil)
		taskSignal.Apply(func(r, c int, v float64) float64 { return 1.0 - v*v }, taskPred)

		prec := rm.taskPrecisionVec()
		taskSignal.MulElem(taskSignal, taskError)
		taskSignal.MulElem(taskSignal, prec)

		update := mat.NewDense(rm.targetDim, rm.arch[topIdx], nil)
		update.Outer(1.0, taskSignal.ColView(0), rm.z[topIdx].ColView(0))

		norm := mat.Norm(update, 2)
		if norm > rm.cfg.GradClip {
			update.Scale(rm.cfg.GradClip/(norm+1e-12), update)
		}

		update.Scale(rm.cfg.LrGenerative, update)
		rm.V.Add(rm.V, update)

		if rm.cfg.WeightDecay > 0 {
			rm.V.Scale(1.0-rm.cfg.LrGenerative*rm.cfg.WeightDecay, rm.V)
		}
	}

	// 4. Update Temporal Prior A
	if rm.prevTop != nil {
		topPrior := mat.NewDense(rm.arch[len(rm.arch)-1], 1, nil)
		topPrior.Mul(rm.A, rm.prevTop)
		topPrior.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, topPrior)

		temporalError := mat.NewDense(rm.arch[len(rm.arch)-1], 1, nil)
		temporalError.Sub(rm.z[len(rm.z)-1], topPrior)

		temporalSignal := mat.NewDense(rm.arch[len(rm.arch)-1], 1, nil)
		temporalSignal.Apply(func(r, c int, v float64) float64 {
			return 1.0 - v*v
		}, topPrior)

		prec := rm.temporalPrecisionVec()
		temporalSignal.MulElem(temporalSignal, temporalError)
		temporalSignal.MulElem(temporalSignal, prec)
		temporalSignal.Scale(rm.cfg.TemporalWeight, temporalSignal)

		update := mat.NewDense(rm.arch[len(rm.arch)-1], rm.arch[len(rm.arch)-1], nil)
		update.Outer(1.0, temporalSignal.ColView(0), rm.prevTop.ColView(0))

		norm := mat.Norm(update, 2)
		if norm > rm.cfg.GradClip {
			update.Scale(rm.cfg.GradClip/(norm+1e-12), update)
		}

		update.Scale(rm.cfg.LrTemporal, update)
		rm.A.Add(rm.A, update)

		if rm.cfg.WeightDecay > 0 {
			rm.A.Scale(1.0-rm.cfg.LrTemporal*rm.cfg.WeightDecay, rm.A)
		}
	}

	rm.updatePrecision(errors, yCol)

	if rm.prevTop == nil {
		rm.prevTop = mat.NewDense(rm.arch[len(rm.arch)-1], 1, nil)
	}
	rm.prevTop.Copy(rm.z[len(rm.z)-1])
}

func (rm *ResonanceManifoldV2) Energy(yCol *mat.Dense) float64 {
	_, errors := rm.predictAdjacentLayers()
	energy := 0.0

	for l, err := range errors {
		prec := rm.precisionFor(l)
		weightedErr := mat.NewDense(rm.arch[l], 1, nil)
		weightedErr.MulElem(prec, err)
		energy += 0.5 * mat.Dot(weightedErr.ColView(0), err.ColView(0))
	}

	if rm.prevTop != nil {
		topPrior := mat.NewDense(rm.arch[len(rm.arch)-1], 1, nil)
		topPrior.Mul(rm.A, rm.prevTop)
		topPrior.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, topPrior)

		temporalError := mat.NewDense(rm.arch[len(rm.arch)-1], 1, nil)
		temporalError.Sub(rm.z[len(rm.z)-1], topPrior)

		prec := rm.temporalPrecisionVec()
		weightedErr := mat.NewDense(rm.arch[len(rm.arch)-1], 1, nil)
		weightedErr.MulElem(prec, temporalError)
		energy += 0.5 * rm.cfg.TemporalWeight * mat.Dot(weightedErr.ColView(0), temporalError.ColView(0))
	}

	// Supervised task energy gate
	if yCol != nil && rm.V != nil {
		topIdx := len(rm.z) - 1
		taskPred := mat.NewDense(rm.targetDim, 1, nil)
		taskPred.Mul(rm.V, rm.z[topIdx])
		taskPred.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, taskPred)

		taskError := mat.NewDense(rm.targetDim, 1, nil)
		taskError.Sub(yCol, taskPred)

		prec := rm.taskPrecisionVec()
		weightedErr := mat.NewDense(rm.targetDim, 1, nil)
		weightedErr.MulElem(prec, taskError)
		energy += 0.5 * mat.Dot(weightedErr.ColView(0), taskError.ColView(0))
	}

	if rm.cfg.LatentDecay > 0 {
		for l := 1; l < len(rm.z); l++ {
			norm := mat.Norm(rm.z[l], 2)
			energy += 0.5 * rm.cfg.LatentDecay * norm * norm
		}
	}

	return energy
}

func (rm *ResonanceManifoldV2) ReconstructionError() float64 {
	pred := mat.NewDense(rm.arch[0], 1, nil)
	pred.Mul(rm.W[0], rm.z[1])
	pred.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, pred)

	diff := mat.NewDense(rm.arch[0], 1, nil)
	diff.Sub(rm.z[0], pred)

	return mat.Norm(diff, 2)
}

func (rm *ResonanceManifoldV2) LatentState() []float64 {
	topZ := rm.z[len(rm.z)-1]
	r, _ := topZ.Dims()
	out := make([]float64, r)
	for i := 0; i < r; i++ {
		out[i] = topZ.At(i, 0)
	}
	return out
}

// ---------------------------- Private Helpers ----------------------------

func (rm *ResonanceManifoldV2) precisionFor(l int) *mat.Dense {
	if !rm.cfg.UsePrecision {
		r, c := rm.precision[l].Dims()
		flat := mat.NewDense(r, c, nil)
		for i := 0; i < r; i++ {
			flat.Set(i, 0, 1.0)
		}
		return flat
	}
	return rm.precision[l]
}

func (rm *ResonanceManifoldV2) temporalPrecisionVec() *mat.Dense {
	if !rm.cfg.UsePrecision {
		r, c := rm.temporalPrecision.Dims()
		flat := mat.NewDense(r, c, nil)
		for i := 0; i < r; i++ {
			flat.Set(i, 0, 1.0)
		}
		return flat
	}
	return rm.temporalPrecision
}

func (rm *ResonanceManifoldV2) taskPrecisionVec() *mat.Dense {
	if !rm.cfg.UsePrecision || rm.taskPrecision == nil {
		flat := mat.NewDense(rm.targetDim, 1, nil)
		for i := 0; i < rm.targetDim; i++ {
			flat.Set(i, 0, 1.0)
		}
		return flat
	}
	return rm.taskPrecision
}

func (rm *ResonanceManifoldV2) predictAdjacentLayers() ([]*mat.Dense, []*mat.Dense) {
	predictions := make([]*mat.Dense, len(rm.W))
	errors := make([]*mat.Dense, len(rm.W))

	for l := 0; l < len(rm.W); l++ {
		pred := mat.NewDense(rm.arch[l], 1, nil)
		pred.Mul(rm.W[l], rm.z[l+1])
		pred.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, pred)
		predictions[l] = pred

		err := mat.NewDense(rm.arch[l], 1, nil)
		err.Sub(rm.z[l], pred)
		errors[l] = err
	}

	return predictions, errors
}

func (rm *ResonanceManifoldV2) initializeLatents(xCol *mat.Dense) {
	// Bottom-up recognition proposal
	bottomUp := make([]*mat.Dense, len(rm.z))
	bottomUp[0] = mat.NewDense(rm.arch[0], 1, nil)
	bottomUp[0].Copy(xCol)

	for l := 0; l < len(rm.R); l++ {
		prop := mat.NewDense(rm.arch[l+1], 1, nil)
		prop.Mul(rm.R[l], bottomUp[l])
		prop.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, prop)
		bottomUp[l+1] = prop
	}

	rm.z[0].Copy(xCol)

	if rm.prevTop == nil {
		for l := 1; l < len(rm.z); l++ {
			rm.z[l].Copy(bottomUp[l])
		}
		return
	}

	// Top-down prior proposal
	topPrior := mat.NewDense(rm.arch[len(rm.arch)-1], 1, nil)
	topPrior.Mul(rm.A, rm.prevTop)
	topPrior.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, topPrior)

	topDown := make([]*mat.Dense, len(rm.z))
	topDown[len(topDown)-1] = topPrior

	for l := len(rm.W) - 1; l > 0; l-- {
		prop := mat.NewDense(rm.arch[l], 1, nil)
		prop.Mul(rm.W[l], topDown[l+1])
		prop.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, prop)
		topDown[l] = prop
	}

	// Merge proposals via top_down_init_mix
	alpha := rm.cfg.TopDownInitMix
	for l := 1; l < len(rm.z); l++ {
		td := mat.NewDense(rm.arch[l], 1, nil)
		td.Scale(alpha, topDown[l])

		bu := mat.NewDense(rm.arch[l], 1, nil)
		bu.Scale(1.0-alpha, bottomUp[l])

		merged := mat.NewDense(rm.arch[l], 1, nil)
		merged.Add(td, bu)

		merged.Apply(func(r, c int, v float64) float64 {
			return math.Min(rm.cfg.StateClip, math.Max(-rm.cfg.StateClip, v))
		}, merged)

		rm.z[l].Copy(merged)
	}
}

func (rm *ResonanceManifoldV2) stateGradients(predictions []*mat.Dense, errors []*mat.Dense, yCol *mat.Dense) []*mat.Dense {
	topIdx := len(rm.z) - 1
	grads := make([]*mat.Dense, len(rm.z))

	for l := 1; l <= topIdx; l++ {
		grad := mat.NewDense(rm.arch[l], 1, nil)

		if l < topIdx {
			prec := rm.precisionFor(l)
			weightedErr := mat.NewDense(rm.arch[l], 1, nil)
			weightedErr.MulElem(prec, errors[l])
			grad.Add(grad, weightedErr)
		}

		belowSignal := mat.NewDense(rm.arch[l-1], 1, nil)
		belowSignal.Apply(func(r, c int, v float64) float64 {
			return 1.0 - v*v
		}, predictions[l-1])

		precBelow := rm.precisionFor(l - 1)
		belowSignal.MulElem(belowSignal, errors[l-1])
		belowSignal.MulElem(belowSignal, precBelow)

		correction := mat.NewDense(rm.arch[l], 1, nil)
		correction.Mul(rm.W[l-1].T(), belowSignal)
		grad.Sub(grad, correction)

		// Supervised task gradients for top latent layer
		if l == topIdx && yCol != nil && rm.V != nil {
			taskPred := mat.NewDense(rm.targetDim, 1, nil)
			taskPred.Mul(rm.V, rm.z[topIdx])
			taskPred.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, taskPred)

			taskError := mat.NewDense(rm.targetDim, 1, nil)
			taskError.Sub(yCol, taskPred)

			taskSignal := mat.NewDense(rm.targetDim, 1, nil)
			taskSignal.Apply(func(r, c int, v float64) float64 { return 1.0 - v*v }, taskPred)

			prec := rm.taskPrecisionVec()
			taskSignal.MulElem(taskSignal, taskError)
			taskSignal.MulElem(taskSignal, prec)

			taskCorrection := mat.NewDense(rm.arch[topIdx], 1, nil)
			taskCorrection.Mul(rm.V.T(), taskSignal)

			grad.Sub(grad, taskCorrection)
		}

		if l == topIdx && rm.prevTop != nil {
			topPrior := mat.NewDense(rm.arch[topIdx], 1, nil)
			topPrior.Mul(rm.A, rm.prevTop)
			topPrior.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, topPrior)

			temporalError := mat.NewDense(rm.arch[topIdx], 1, nil)
			temporalError.Sub(rm.z[topIdx], topPrior)

			precTemp := rm.temporalPrecisionVec()
			temporalError.MulElem(temporalError, precTemp)
			temporalError.Scale(rm.cfg.TemporalWeight, temporalError)

			grad.Add(grad, temporalError)
		}

		if rm.cfg.LatentDecay > 0 {
			decayTerm := mat.NewDense(rm.arch[l], 1, nil)
			decayTerm.Scale(rm.cfg.LatentDecay, rm.z[l])
			grad.Add(grad, decayTerm)
		}

		gradNorm := mat.Norm(grad, 2)
		if gradNorm > rm.cfg.GradClip {
			grad.Scale(rm.cfg.GradClip/(gradNorm+1e-12), grad)
		}

		grads[l] = grad
	}

	return grads
}

func (rm *ResonanceManifoldV2) applyStateUpdate(baseStates []*mat.Dense, grads []*mat.Dense, stepSize float64) []*mat.Dense {
	candidate := make([]*mat.Dense, len(baseStates))
	candidate[0] = mat.NewDense(rm.arch[0], 1, nil)
	candidate[0].Copy(baseStates[0])

	for l := 1; l < len(baseStates); l++ {
		step := mat.NewDense(rm.arch[l], 1, nil)
		step.Scale(stepSize, grads[l])

		nextState := mat.NewDense(rm.arch[l], 1, nil)
		nextState.Sub(baseStates[l], step)
		nextState.Apply(func(r, c int, v float64) float64 {
			return math.Min(rm.cfg.StateClip, math.Max(-rm.cfg.StateClip, v))
		}, nextState)

		candidate[l] = nextState
	}

	return candidate
}

func (rm *ResonanceManifoldV2) updatePrecision(errors []*mat.Dense, yCol *mat.Dense) {
	if !rm.cfg.UsePrecision {
		return
	}

	beta := rm.cfg.PrecisionBeta

	for l, err := range errors {
		r, _ := rm.errorVar[l].Dims()
		for i := 0; i < r; i++ {
			errVal := err.At(i, 0)
			v := rm.errorVar[l].At(i, 0)
			v = (1.0-beta)*v + beta*(errVal*errVal)
			rm.errorVar[l].Set(i, 0, v)

			rawPrecision := 1.0 / (v + rm.cfg.PrecisionEps)
			precisionVal := math.Min(rm.cfg.PrecisionMax, math.Max(rm.cfg.PrecisionMin, rawPrecision))
			rm.precision[l].Set(i, 0, precisionVal)
		}
	}

	if rm.prevTop != nil {
		topPrior := mat.NewDense(rm.arch[len(rm.arch)-1], 1, nil)
		topPrior.Mul(rm.A, rm.prevTop)
		topPrior.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, topPrior)

		temporalError := mat.NewDense(rm.arch[len(rm.arch)-1], 1, nil)
		temporalError.Sub(rm.z[len(rm.z)-1], topPrior)

		r, _ := rm.temporalVar.Dims()
		for i := 0; i < r; i++ {
			errVal := temporalError.At(i, 0)
			v := rm.temporalVar.At(i, 0)
			v = (1.0-beta)*v + beta*(errVal*errVal)
			rm.temporalVar.Set(i, 0, v)

			rawPrecision := 1.0 / (v + rm.cfg.PrecisionEps)
			precisionVal := math.Min(rm.cfg.PrecisionMax, math.Max(rm.cfg.PrecisionMin, rawPrecision))
			rm.temporalPrecision.Set(i, 0, precisionVal)
		}
	}

	if yCol != nil && rm.V != nil {
		topIdx := len(rm.z) - 1
		taskPred := mat.NewDense(rm.targetDim, 1, nil)
		taskPred.Mul(rm.V, rm.z[topIdx])
		taskPred.Apply(func(r, c int, v float64) float64 { return math.Tanh(v) }, taskPred)

		taskError := mat.NewDense(rm.targetDim, 1, nil)
		taskError.Sub(yCol, taskPred)

		r, _ := rm.taskVar.Dims()
		for i := 0; i < r; i++ {
			errVal := taskError.At(i, 0)
			v := rm.taskVar.At(i, 0)
			v = (1.0-beta)*v + beta*(errVal*errVal)
			rm.taskVar.Set(i, 0, v)

			rawPrecision := 1.0 / (v + rm.cfg.PrecisionEps)
			precisionVal := math.Min(rm.cfg.PrecisionMax, math.Max(rm.cfg.PrecisionMin, rawPrecision))
			rm.taskPrecision.Set(i, 0, precisionVal)
		}
	}
}

func copyStates(src []*mat.Dense) []*mat.Dense {
	dst := make([]*mat.Dense, len(src))
	for i, s := range src {
		r, c := s.Dims()
		dst[i] = mat.NewDense(r, c, nil)
		dst[i].Copy(s)
	}
	return dst
}
