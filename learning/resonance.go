package learning

import (
	"errors"
	"fmt"
	"math"
	"math/rand"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/mat"
)

type ResonanceConfig struct {
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

	TemporalWeight  float64
	TopDownInitMix  float64
	TemporalNormMax float64

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

Two families of parameter live in here and they do not behave the same way under
a later SetAlpha. Pace terms (the learning rates, the precision tracking weight,
the regularizer strengths) are what alpha is for, and retuning them mid-stream is
the intended effect. Geometry terms (StateClip, TopDownInitMix, the inference
step counts) describe the shape of the state space the retained weights and
precisions were estimated in; moving those mid-stream changes the problem rather
than the pace at which it is solved. SetAlpha therefore re-derives only the pace
family. See ResonanceConfig.adoptPace.
*/
func AdaptiveResonanceConfig(
	alpha float64, arch []int,
) ResonanceConfig {
	depth := len(arch)
	depthFloat := float64(depth)

	topDownInitMix := (depthFloat - 1.0) / depthFloat
	temporalNormMax := 1.0 - 1.0/depthFloat
	temporalWeight := alpha / (alpha + 1.0/depthFloat)
	earlyStopPatience := int(math.Max(1, math.Ceil(math.Sqrt(depthFloat))))
	gradClip := alpha * depthFloat

	// StateClip bounds the latent magnitude. Deriving it as depth/alpha makes a
	// faster learning pace imply a tighter state space, which is backwards and
	// couples the clip to the controller: a pace that rose by 30x would shrink
	// the admissible latent range by the same factor and clip states that the
	// retained weights were fit against. The bound belongs to the activation
	// geometry instead. Every latent below the top is a tanh image bounded by 1,
	// and the merge in initializeLatents is a convex combination of two such
	// images, so a bound of depth admits the full reachable range with room for
	// the transient excursions inference makes on the way to a settled state,
	// and is invariant to pace.
	stateClip := depthFloat

	return ResonanceConfig{
		MaxInferenceSteps:  depth * 8,
		MinInferenceSteps:  depth * 2,
		LrState:            alpha * 10.0,
		EarlyStopTol:       1e-5,
		EarlyStopPatience:  earlyStopPatience,
		MonotoneStateSteps: true,
		LineSearchHalvings: 3,

		LrGenerative:  alpha * 1.0,
		LrTemporal:    alpha * 2.0,
		LrRecognition: alpha * 0.6,

		TemporalWeight:  temporalWeight,
		TopDownInitMix:  topDownInitMix,
		TemporalNormMax: temporalNormMax,

		UsePrecision:  true,
		PrecisionBeta: alpha,
		PrecisionMin:  0.10,
		PrecisionMax:  5.0,
		PrecisionEps:  1e-4,

		LatentDecay: alpha * 1e-1,
		Sparsity:    alpha * 1e-2,
		WeightDecay: alpha * 1e-3,
		GradClip:    gradClip,
		StateClip:   stateClip,
	}
}

type ResonanceManifold struct {
	cfg                    ResonanceConfig
	arch                   []int
	targetDim              int
	W                      []*mat.Dense
	R                      []*mat.Dense
	A                      *mat.Dense
	V                      *mat.Dense
	taskBias               *mat.Dense
	taskLearners           []*RLS
	z                      []*mat.Dense
	prevTop                *mat.Dense
	errorVar               []*mat.Dense
	precision              []*mat.Dense
	temporalVar            *mat.Dense
	temporalPrecision      *mat.Dense
	temporalPrior          *mat.Dense
	temporalPriorReady     bool
	settleAdvancedTemporal bool
	taskVar                *mat.Dense
	taskScale              *mat.Dense
	taskScaleReady         []bool
	taskPrecision          *mat.Dense
	taskBaselineMean       *mat.Dense
	taskModelLoss          *mat.Dense
	taskBaselineLoss       *mat.Dense
	taskBaselineReady      []bool
	taskSkillReady         []bool
	taskSkill              *mat.Dense
	workspace              *resonanceWorkspace
	streamLearn            bool
	streamAdvanceTemporal  bool
	lastInferenceSteps     int
	output                 float64
}

func NewResonanceManifold(
	arch []int, targetDim int, alpha float64,
) (*ResonanceManifold, error) {
	if len(arch) < 2 {
		return nil, errors.New("resonance: architecture must contain at least input and one latent layer")
	}

	if alpha <= 0 || alpha > 1 || math.IsNaN(alpha) || math.IsInf(alpha, 0) {
		return nil, errors.New("resonance: alpha must be finite and in (0, 1]")
	}

	cfg := AdaptiveResonanceConfig(alpha, arch)
	rng := rand.New(rand.NewSource(42))
	numLinks := len(arch) - 1

	weights := make([]*mat.Dense, numLinks)
	recognition := make([]*mat.Dense, numLinks)
	errorVar := make([]*mat.Dense, numLinks)
	precision := make([]*mat.Dense, numLinks)

	for layerIndex := 0; layerIndex < numLinks; layerIndex++ {
		rowCount, colCount := arch[layerIndex], arch[layerIndex+1]
		scaleW := math.Sqrt(2.0 / float64(rowCount+colCount))
		dataW := make([]float64, rowCount*colCount)

		for index := range dataW {
			dataW[index] = rng.NormFloat64() * scaleW
		}

		weights[layerIndex] = mat.NewDense(rowCount, colCount, dataW)
		scaleR := math.Sqrt(2.0 / float64(rowCount+colCount))
		dataR := make([]float64, colCount*rowCount)

		for index := range dataR {
			dataR[index] = rng.NormFloat64() * scaleR
		}

		recognition[layerIndex] = mat.NewDense(colCount, rowCount, dataR)
		errorVar[layerIndex] = mat.NewDense(rowCount, 1, nil)
		precision[layerIndex] = mat.NewDense(rowCount, 1, nil)

		for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
			errorVar[layerIndex].Set(rowIndex, 0, 1.0)
			precision[layerIndex].Set(rowIndex, 0, 1.0)
		}
	}

	topDim := arch[len(arch)-1]
	scaleA := math.Sqrt(1.0 / float64(topDim))
	dataA := make([]float64, topDim*topDim)

	for index := range dataA {
		dataA[index] = rng.NormFloat64() * scaleA * 0.30
	}

	temporalWeights := mat.NewDense(topDim, topDim, dataA)

	var taskWeights *mat.Dense
	var taskBias *mat.Dense
	var taskLearners []*RLS
	var taskVar *mat.Dense
	var taskScale *mat.Dense
	var taskScaleReady []bool
	var taskPrecision *mat.Dense
	var taskBaselineMean *mat.Dense
	var taskModelLoss *mat.Dense
	var taskBaselineLoss *mat.Dense
	var taskBaselineReady []bool
	var taskSkillReady []bool
	var taskSkill *mat.Dense

	if targetDim > 0 {
		// The head is linear and fits a target on the caller's own scale, so it
		// starts at zero rather than at a random draw. A random head is a
		// nonzero forecast asserted before a single sample has been seen, and
		// with the small-magnitude targets this head is built for that noise
		// can exceed the signal it will eventually learn. Zero forecasts
		// nothing until the data says otherwise, and the top latent it reads
		// from is already randomly projected, so no symmetry needs breaking
		// here.
		taskWeights = mat.NewDense(targetDim, topDim, nil)
		taskBias = mat.NewDense(targetDim, 1, nil)
		taskLearners = make([]*RLS, targetDim)
		taskVar = mat.NewDense(targetDim, 1, nil)
		taskScale = mat.NewDense(targetDim, 1, nil)
		taskScaleReady = make([]bool, targetDim)
		taskPrecision = mat.NewDense(targetDim, 1, nil)
		taskBaselineMean = mat.NewDense(targetDim, 1, nil)
		taskModelLoss = mat.NewDense(targetDim, 1, nil)
		taskBaselineLoss = mat.NewDense(targetDim, 1, nil)
		taskBaselineReady = make([]bool, targetDim)
		taskSkillReady = make([]bool, targetDim)
		taskSkill = mat.NewDense(targetDim, 1, nil)

		for rowIndex := range targetDim {
			learner, err := NewRLS(RLSConfig{
				Dimension:       topDim,
				InitialVariance: 1,
			})

			if err != nil {
				return nil, fmt.Errorf("resonance: construct task learner: %w", err)
			}

			taskLearners[rowIndex] = learner
			taskVar.Set(rowIndex, 0, 1.0)

			// taskScale holds a log variance, so zero is unit scale. It is
			// replaced outright by the first observation regardless.
			taskScale.Set(rowIndex, 0, 0.0)
			taskPrecision.Set(rowIndex, 0, 1.0)
			taskSkill.Set(rowIndex, 0, 1.0)
		}
	}

	latents := make([]*mat.Dense, len(arch))

	for layerIndex, layerDim := range arch {
		latents[layerIndex] = mat.NewDense(layerDim, 1, nil)
	}

	temporalVar := mat.NewDense(topDim, 1, nil)
	temporalPrecision := mat.NewDense(topDim, 1, nil)

	for rowIndex := range topDim {
		temporalVar.Set(rowIndex, 0, 1.0)
		temporalPrecision.Set(rowIndex, 0, 1.0)
	}

	manifold := &ResonanceManifold{
		cfg:                   cfg,
		arch:                  arch,
		targetDim:             targetDim,
		W:                     weights,
		R:                     recognition,
		A:                     temporalWeights,
		V:                     taskWeights,
		taskBias:              taskBias,
		taskLearners:          taskLearners,
		z:                     latents,
		errorVar:              errorVar,
		precision:             precision,
		temporalVar:           temporalVar,
		temporalPrecision:     temporalPrecision,
		temporalPrior:         mat.NewDense(topDim, 1, nil),
		taskVar:               taskVar,
		taskScale:             taskScale,
		taskScaleReady:        taskScaleReady,
		taskPrecision:         taskPrecision,
		taskBaselineMean:      taskBaselineMean,
		taskModelLoss:         taskModelLoss,
		taskBaselineLoss:      taskBaselineLoss,
		taskBaselineReady:     taskBaselineReady,
		taskSkillReady:        taskSkillReady,
		taskSkill:             taskSkill,
		workspace:             newResonanceWorkspace(arch, targetDim),
		streamLearn:           true,
		streamAdvanceTemporal: true,
	}

	manifold.constrainTemporalWeights()
	return manifold, nil
}

func (rm *ResonanceManifold) ResetState(resetPrecision bool) {
	for _, latent := range rm.z {
		rowCount, _ := latent.Dims()
		for rowIndex := range rowCount {
			latent.Set(rowIndex, 0, 0.0)
		}
	}
	rm.prevTop = nil
	rm.temporalPriorReady = false
	rm.settleAdvancedTemporal = false

	if resetPrecision {
		for layerIndex := 0; layerIndex < len(rm.W); layerIndex++ {
			rowCount, _ := rm.errorVar[layerIndex].Dims()
			for rowIndex := range rowCount {
				rm.errorVar[layerIndex].Set(rowIndex, 0, 1.0)
				rm.precision[layerIndex].Set(rowIndex, 0, 1.0)
			}
		}
		topDim := rm.arch[len(rm.arch)-1]
		for rowIndex := range topDim {
			rm.temporalVar.Set(rowIndex, 0, 1.0)
			rm.temporalPrecision.Set(rowIndex, 0, 1.0)
		}
		if rm.targetDim > 0 {
			for rowIndex := 0; rowIndex < rm.targetDim; rowIndex++ {
				rm.taskVar.Set(rowIndex, 0, 1.0)
				rm.taskScale.Set(rowIndex, 0, 0.0)
				rm.taskPrecision.Set(rowIndex, 0, 1.0)
				rm.taskBaselineMean.Set(rowIndex, 0, 0.0)
				rm.taskModelLoss.Set(rowIndex, 0, 0.0)
				rm.taskBaselineLoss.Set(rowIndex, 0, 0.0)
				rm.taskSkill.Set(rowIndex, 0, 1.0)
				rm.taskScaleReady[rowIndex] = false
				rm.taskBaselineReady[rowIndex] = false
				rm.taskSkillReady[rowIndex] = false
			}
		}
	}
}

func (rm *ResonanceManifold) SetStreamLearn(enabled bool) {
	rm.streamLearn = enabled
}

func (rm *ResonanceManifold) SetStreamAdvanceTemporal(enabled bool) {
	rm.streamAdvanceTemporal = enabled
}

func (rm *ResonanceManifold) SettleFromBatch(input []float64, target []float64) (float64, error) {
	return rm.SettleFromBatchOptions(input, target, rm.streamLearn, rm.streamAdvanceTemporal)
}

func (rm *ResonanceManifold) SettleFromBatchOptions(
	input []float64,
	target []float64,
	learn bool,
	advanceTemporal bool,
) (float64, error) {
	if len(input) != rm.arch[0] {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"resonance: input dimension mismatch",
			errors.New("resonance: input dimension mismatch"),
		))
	}

	settleAdvanceTemporal := advanceTemporal && !learn
	err := rm.Settle(input, settleAdvanceTemporal)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"resonance: settle failed",
			err,
		))
	}

	if learn {
		if err := rm.Learn(target); err != nil {
			return 0, err
		}
	}

	reconstruction := rm.ReconstructionError()
	rm.output = reconstruction

	return reconstruction, nil
}

func (rm *ResonanceManifold) ReconstructionOutput() float64 {
	return rm.output
}

/*
Settle performs generative inference without supervised target contamination.
Supervised targets belong in Learn and only affect weight updates.
*/
func (rm *ResonanceManifold) Settle(input []float64, advanceTemporal bool) error {
	if len(input) != rm.arch[0] {
		return errors.New("resonance: input dimension mismatch")
	}

	rm.settleAdvancedTemporal = false
	rm.lastInferenceSteps = 0

	xCol := rm.workspace.xCol
	for rowIndex, value := range input {
		xCol.Set(rowIndex, 0, value)
	}

	rm.initializeLatents(xCol)
	rm.temporalPriorReady = rm.prevTop != nil

	if rm.temporalPriorReady {
		rm.temporalPrior.Copy(rm.workspace.topPrior)
	}

	trace := make([]float64, 0, rm.cfg.MaxInferenceSteps+2)
	trace = append(trace, rm.Energy())
	stableSteps := 0

	for step := 0; step < rm.cfg.MaxInferenceSteps; step++ {
		rm.lastInferenceSteps = step + 1
		predictions, layerErrors := rm.predictAdjacentLayers()
		gradients := rm.stateGradients(predictions, layerErrors)

		rm.saveStates()
		oldEnergy := trace[len(trace)-1]
		accepted := false
		stepSize := rm.cfg.LrState

		halvings := 0
		if rm.cfg.MonotoneStateSteps {
			halvings = rm.cfg.LineSearchHalvings
		}

		for halvingIndex := 0; halvingIndex <= halvings; halvingIndex++ {
			rm.tryStateUpdate(gradients, stepSize)
			rm.z[0].Copy(xCol)
			newEnergy := rm.Energy()

			if !rm.cfg.MonotoneStateSteps ||
				newEnergy <= math.Nextafter(oldEnergy, math.Inf(1)) {
				accepted = true
				break
			}

			rm.restoreStates()
			stepSize *= 0.5
		}

		if !accepted {
			rm.restoreStates()
			rm.z[0].Copy(xCol)
			trace = append(trace, oldEnergy)
			stableSteps = 0

			continue
		}

		trace = append(trace, rm.Energy())

		deltaEnergy := math.Abs(trace[len(trace)-2] - trace[len(trace)-1])
		energyScale := math.Max(math.Abs(trace[len(trace)-2]), rm.cfg.PrecisionEps)
		relativeDelta := deltaEnergy / energyScale

		if step+1 < rm.cfg.MinInferenceSteps || relativeDelta >= rm.cfg.EarlyStopTol {
			stableSteps = 0

			continue
		}

		stableSteps++

		if stableSteps >= rm.cfg.EarlyStopPatience {
			break
		}
	}

	if advanceTemporal {
		rm.advanceTemporalState()
		rm.settleAdvancedTemporal = true
	}

	return nil
}

func (rm *ResonanceManifold) Learn(target []float64) error {
	if rm.settleAdvancedTemporal {
		return errors.New("resonance: temporal state advanced before learning")
	}

	if target != nil && len(target) != rm.targetDim {
		return fmt.Errorf(
			"resonance: target dimension mismatch: expected %d, got %d",
			rm.targetDim,
			len(target),
		)
	}

	predictions, layerErrors := rm.predictAdjacentLayers()
	topIndex := len(rm.z) - 1

	var targetCol *mat.Dense
	if target != nil && rm.targetDim > 0 {
		targetCol = rm.workspace.yCol
		for rowIndex, value := range target {
			targetCol.Set(rowIndex, 0, value)
		}
	}

	for layerIndex, weightMatrix := range rm.W {
		localSignal := rm.workspace.localSignal[layerIndex]
		denseApplyOneMinusSquareInto(localSignal, predictions[layerIndex])

		precision := rm.precisionFor(layerIndex)
		localSignal.MulElem(localSignal, layerErrors[layerIndex])
		localSignal.MulElem(localSignal, precision)

		update := rm.workspace.weightUpdate[layerIndex]
		denseOuterColsInto(update, localSignal, rm.z[layerIndex+1], 1.0)

		norm := mat.Norm(update, 2)
		if norm > rm.cfg.GradClip {
			update.Scale(rm.cfg.GradClip/norm, update)
		}

		update.Scale(rm.cfg.LrGenerative, update)
		weightMatrix.Add(weightMatrix, update)

		if rm.cfg.WeightDecay > 0 {
			weightMatrix.Scale(1.0-rm.cfg.LrGenerative*rm.cfg.WeightDecay, weightMatrix)
		}
	}

	for layerIndex, recognitionMatrix := range rm.R {
		proposal := rm.workspace.recProposal[layerIndex]
		proposal.Mul(recognitionMatrix, rm.z[layerIndex])
		denseApplyTanhInPlace(proposal)

		recError := rm.workspace.recError[layerIndex]
		recError.Sub(rm.z[layerIndex+1], proposal)

		recSignal := rm.workspace.recSignal[layerIndex]
		denseApplyOneMinusSquareInto(recSignal, proposal)
		recSignal.MulElem(recSignal, recError)

		update := rm.workspace.recUpdate[layerIndex]
		denseOuterColsInto(update, recSignal, rm.z[layerIndex], 1.0)

		norm := mat.Norm(update, 2)
		if norm > rm.cfg.GradClip {
			update.Scale(rm.cfg.GradClip/norm, update)
		}

		update.Scale(rm.cfg.LrRecognition, update)
		recognitionMatrix.Add(recognitionMatrix, update)

		if rm.cfg.WeightDecay > 0 {
			recognitionMatrix.Scale(1.0-rm.cfg.LrRecognition*rm.cfg.WeightDecay, recognitionMatrix)
		}
	}

	var taskError *mat.Dense

	if targetCol != nil && rm.V != nil {
		taskPred := rm.workspace.taskPred
		rm.taskPredictionInto(taskPred)

		taskError = rm.workspace.taskError
		taskError.Sub(targetCol, taskPred)

		/*
			The return head is a linear regression, so square-root recursive least
			squares is the exact online learner for its objective. Its gain follows
			from retained design covariance; sharing the manifold's scalar alpha
			with unrelated generative, recognition, temporal, precision, and
			regularization updates made the task fit move for reasons outside its
			own forecast error.

			Initial covariance is the identity because the top latent is a tanh
			image on a unit scale. Forgetting is one: no evidence is discarded
			without an observed regime-reset rule.
		*/
		topState := rm.z[topIndex].RawMatrix().Data

		for rowIndex, learner := range rm.taskLearners {
			_, err := learner.Observe(RLSSample{
				Features: topState,
				Target:   targetCol.At(rowIndex, 0),
			})

			if err != nil {
				return fmt.Errorf("resonance: task learner update: %w", err)
			}

			intercept, err := learner.copyCoefficients(rm.V.RawRowView(rowIndex))

			if err != nil {
				return fmt.Errorf("resonance: task learner coefficients: %w", err)
			}

			rm.taskBias.Set(rowIndex, 0, intercept)
		}
	}

	var temporalError *mat.Dense

	if rm.temporalPriorReady {
		temporalError = rm.workspace.temporalError
		temporalError.Sub(rm.z[topIndex], rm.temporalPrior)

		temporalSignal := rm.workspace.temporalSignal
		denseApplyOneMinusSquareInto(temporalSignal, rm.temporalPrior)

		precision := rm.temporalPrecisionVec()
		temporalSignal.MulElem(temporalSignal, temporalError)
		temporalSignal.MulElem(temporalSignal, precision)
		temporalSignal.Scale(rm.cfg.TemporalWeight, temporalSignal)

		update := rm.workspace.temporalUpdate
		denseOuterColsInto(update, temporalSignal, rm.prevTop, 1.0)

		norm := mat.Norm(update, 2)
		if norm > rm.cfg.GradClip {
			update.Scale(rm.cfg.GradClip/norm, update)
		}

		update.Scale(rm.cfg.LrTemporal, update)
		rm.A.Add(rm.A, update)

		if rm.cfg.WeightDecay > 0 {
			rm.A.Scale(1.0-rm.cfg.LrTemporal*rm.cfg.WeightDecay, rm.A)
		}

		rm.constrainTemporalWeights()
	}

	if err := rm.updatePrecision(layerErrors, temporalError, targetCol, taskError); err != nil {
		return err
	}

	rm.advanceTemporalState()
	return nil
}

/*
Energy is the variational objective the inference line search minimizes. It is
the sum of precision-weighted prediction error and the regularizer penalties
that shape the latent state.

Do not report this as a measure of how well the network is predicting. The
regularizer terms are a function of the latent magnitudes and of alpha, not of
market surprise, so a pace change moves this number without any change in
prediction quality. PredictionEnergy isolates the part that is prediction error.
*/
func (rm *ResonanceManifold) Energy() float64 {
	energy := rm.PredictionEnergy()

	if rm.cfg.LatentDecay > 0 {
		for layerIndex := 1; layerIndex < len(rm.z); layerIndex++ {
			norm := denseColNorm(rm.z[layerIndex])
			energy += 0.5 * rm.cfg.LatentDecay * norm * norm
		}
	}

	if rm.cfg.Sparsity > 0 {
		for layerIndex := 1; layerIndex < len(rm.z); layerIndex++ {
			rowCount, _ := rm.z[layerIndex].Dims()

			for rowIndex := range rowCount {
				energy += rm.cfg.Sparsity * math.Abs(rm.z[layerIndex].At(rowIndex, 0))
			}
		}
	}

	return energy
}

/*
PredictionEnergy is the precision-weighted prediction error alone: the
generative residual at every link plus the temporal residual at the top,
excluding the latent-decay and sparsity penalties that Energy adds.

This is the quantity to report and to compare across ticks. It responds only to
how well the network predicts, so unlike Energy it does not move when the
learning pace is retuned.
*/
func (rm *ResonanceManifold) PredictionEnergy() float64 {
	_, layerErrors := rm.predictAdjacentLayers()
	energy := 0.0

	for layerIndex, layerError := range layerErrors {
		if rm.cfg.UsePrecision {
			weightedError := rm.workspace.weightedErr[layerIndex]
			weightedError.MulElem(rm.precisionFor(layerIndex), layerError)
			energy += 0.5 * denseColDot(weightedError, layerError)

			continue
		}

		energy += 0.5 * denseColDot(layerError, layerError)
	}

	if rm.temporalPriorReady {
		temporalError := rm.workspace.temporalError
		temporalError.Sub(rm.z[len(rm.z)-1], rm.temporalPrior)

		if rm.cfg.UsePrecision {
			weightedError := rm.workspace.temporalWeightedErr
			weightedError.MulElem(rm.temporalPrecisionVec(), temporalError)
			energy += 0.5 * rm.cfg.TemporalWeight * denseColDot(weightedError, temporalError)
		} else {
			energy += 0.5 * rm.cfg.TemporalWeight * denseColDot(temporalError, temporalError)
		}
	}

	return energy
}

func (rm *ResonanceManifold) ReconstructionError() float64 {
	reconstruction := rm.workspace.reconPred
	reconstruction.Mul(rm.W[0], rm.z[1])
	denseApplyTanhInPlace(reconstruction)

	diff := rm.workspace.reconDiff
	diff.Sub(rm.z[0], reconstruction)

	return denseColNorm(diff)
}

/*
taskPredictionInto evaluates the supervised head y = V * z into dst.

The head is deliberately linear, unlike every generative and recognition link in
the network, which are tanh. Those links reconstruct latent states that are
themselves bounded tanh images, so squashing them is what makes prediction and
target commensurate. The supervised target is not such a state: it is whatever
scale the caller's regression lives on. For a log return that scale is order
1e-4, which sits so deep inside tanh's linear region that the squash contributes
nothing but a systematically attenuated gradient, while capping the head at +/-1
would silently truncate any caller whose target is larger. A linear head fits the
target on its own scale and leaves saturation to the caller who chose it.
*/
func (rm *ResonanceManifold) taskPredictionInto(dst *mat.Dense) {
	dst.Mul(rm.V, rm.z[len(rm.z)-1])
	dst.Add(dst, rm.taskBias)
}

func (rm *ResonanceManifold) TaskPrediction() []float64 {
	if rm.V == nil || rm.targetDim <= 0 {
		return nil
	}

	taskPred := rm.workspace.taskPred
	rm.taskPredictionInto(taskPred)

	rowCount, _ := taskPred.Dims()
	prediction := make([]float64, rowCount)

	for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
		prediction[rowIndex] = taskPred.At(rowIndex, 0)
	}

	return prediction
}

func (rm *ResonanceManifold) LatentState() []float64 {
	topLatent := rm.z[len(rm.z)-1]
	rowCount, _ := topLatent.Dims()
	output := make([]float64, rowCount)
	for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
		output[rowIndex] = topLatent.At(rowIndex, 0)
	}

	return output
}

/*
ResonanceLayerWire exports one settled layer for UI x-ray visualization.

Temporal reports whether ErrorNorm is a temporal mismatch rather than a
generative one. Only the top layer carries a temporal error, because it is the
only layer no other layer predicts top-down.
*/
type ResonanceLayerWire struct {
	State      []float64 `json:"state"`
	Prediction []float64 `json:"prediction"`
	ErrorNorm  float64   `json:"errorNorm"`
	Temporal   bool      `json:"temporal"`
}

/*
TemporalError reports the norm of the top layer's temporal prediction residual,
z_top - tanh(A * z_prev), and whether that residual is defined at all.

The top latent has no generative error term: layerErrors is indexed by link, of
which there are len(arch)-1, so the top layer is not predicted from above by any
weight matrix. Its prediction error is temporal, carried by A across ticks, and
it is undefined on the very first settle because no prior top state exists yet.
Callers driving a controller off this must honour the ok flag, because a zero
returned as if it were a measurement reads as perfect temporal prediction and
inverts whatever ratio it feeds.
*/
func (rm *ResonanceManifold) TemporalError() (float64, bool) {
	if !rm.temporalPriorReady {
		return 0, false
	}

	temporalError := rm.workspace.temporalError
	temporalError.Sub(rm.z[len(rm.z)-1], rm.temporalPrior)

	return denseColNorm(temporalError), true
}

/*
WireSnapshot exports settled states, top-down predictions, and layer errors.
*/
func (rm *ResonanceManifold) WireSnapshot() (
	layers []ResonanceLayerWire,
	surprise float64,
	energy float64,
) {
	predictions, layerErrors := rm.predictAdjacentLayers()
	layers = make([]ResonanceLayerWire, len(rm.z))
	topIndex := len(rm.z) - 1
	temporalNorm, hasTemporal := rm.TemporalError()

	for layerIndex := range rm.z {
		stateMatrix := rm.z[layerIndex]
		rowCount, _ := stateMatrix.Dims()
		state := make([]float64, rowCount)
		prediction := make([]float64, rowCount)

		for rowIndex := range rowCount {
			state[rowIndex] = stateMatrix.At(rowIndex, 0)

			if layerIndex < len(predictions) {
				prediction[rowIndex] = predictions[layerIndex].At(rowIndex, 0)
			}

			if layerIndex == topIndex && rm.temporalPriorReady {
				prediction[rowIndex] = rm.temporalPrior.At(rowIndex, 0)
			}
		}

		errorNorm := 0.0
		temporal := false

		switch {
		case layerIndex < len(layerErrors):
			errorNorm = denseColNorm(layerErrors[layerIndex])
		case layerIndex == topIndex && hasTemporal:
			// The top layer's only residual is the temporal one. Reporting it
			// here is what makes the wire's last row a measurement rather than
			// a structural zero.
			errorNorm = temporalNorm
			temporal = true
		}

		layers[layerIndex] = ResonanceLayerWire{
			State:      state,
			Prediction: prediction,
			ErrorNorm:  errorNorm,
			Temporal:   temporal,
		}
	}

	return layers, rm.ReconstructionError(), rm.Energy()
}

func (rm *ResonanceManifold) advanceTemporalState() {
	topIndex := len(rm.z) - 1

	if rm.prevTop == nil {
		rm.prevTop = mat.NewDense(rm.arch[topIndex], 1, nil)
	}

	rm.prevTop.Copy(rm.z[topIndex])
}

func (rm *ResonanceManifold) precisionFor(layerIndex int) *mat.Dense {
	return rm.precision[layerIndex]
}

func (rm *ResonanceManifold) temporalPrecisionVec() *mat.Dense {
	return rm.temporalPrecision
}

/*
TaskPrecision reports how reliable the supervised head currently is, relative to
its own retained residual scale, and whether it has seen enough to say.

The value is scale-free by construction: one means the head is predicting at its
typical accuracy, above one means it is currently doing better than its own
history, below one means worse. That makes it the quantity a caller should use
to decide how far ahead to trust the head, because it means the same thing
whatever scale the caller's target is on and whatever the market is doing.

ok is false before any supervised sample has been resolved, when the head has no
basis for a claim at all.
*/
func (rm *ResonanceManifold) TaskPrecision() (float64, bool) {
	if rm.V == nil || rm.targetDim <= 0 {
		return 0, false
	}

	total := 0.0

	for rowIndex := range rm.targetDim {
		if !rm.taskScaleReady[rowIndex] {
			return 0, false
		}

		total += rm.taskPrecision.At(rowIndex, 0)
	}

	return total / float64(rm.targetDim), true
}

/*
TaskSkill reports the supervised head's scale-free prequential skill against an
online-mean baseline. Values above one mean the head has lower retained squared
error than the baseline; values below one mean the baseline is better.
*/
func (rm *ResonanceManifold) TaskSkill() (float64, bool) {
	if rm.V == nil || rm.targetDim <= 0 {
		return 0, false
	}

	total := 0.0

	for rowIndex := range rm.targetDim {
		if !rm.taskSkillReady[rowIndex] {
			return 0, false
		}

		total += rm.taskSkill.At(rowIndex, 0)
	}

	return total / float64(rm.targetDim), true
}

func (rm *ResonanceManifold) predictAdjacentLayers() ([]*mat.Dense, []*mat.Dense) {
	for layerIndex := 0; layerIndex < len(rm.W); layerIndex++ {
		prediction := rm.workspace.predictions[layerIndex]
		prediction.Mul(rm.W[layerIndex], rm.z[layerIndex+1])
		denseApplyTanhInPlace(prediction)

		layerError := rm.workspace.errors[layerIndex]
		layerError.Sub(rm.z[layerIndex], prediction)
	}

	return rm.workspace.predictions, rm.workspace.errors
}

func (rm *ResonanceManifold) initializeLatents(xCol *mat.Dense) {
	bottomUp := rm.workspace.bottomUp
	bottomUp[0].Copy(xCol)

	for layerIndex := 0; layerIndex < len(rm.R); layerIndex++ {
		proposal := bottomUp[layerIndex+1]
		proposal.Mul(rm.R[layerIndex], bottomUp[layerIndex])
		denseApplyTanhInPlace(proposal)
	}

	rm.z[0].Copy(xCol)

	if rm.prevTop == nil {
		for layerIndex := 1; layerIndex < len(rm.z); layerIndex++ {
			rm.z[layerIndex].Copy(bottomUp[layerIndex])
		}

		return
	}

	topPrior := rm.workspace.topPrior
	topPrior.Mul(rm.A, rm.prevTop)

	denseApplyTanhInPlace(topPrior)

	topDown := rm.workspace.topDown
	topDown[len(topDown)-1].Copy(topPrior)

	for layerIndex := len(rm.W) - 1; layerIndex > 0; layerIndex-- {
		proposal := topDown[layerIndex]
		proposal.Mul(rm.W[layerIndex], topDown[layerIndex+1])
		denseApplyTanhInPlace(proposal)
	}

	initMix := rm.cfg.TopDownInitMix

	for layerIndex := 1; layerIndex < len(rm.z); layerIndex++ {
		topDownTerm := rm.workspace.mergeTD[layerIndex]
		topDownTerm.Scale(initMix, topDown[layerIndex])

		bottomUpTerm := rm.workspace.mergeBU[layerIndex]
		bottomUpTerm.Scale(1.0-initMix, bottomUp[layerIndex])

		merged := rm.z[layerIndex]
		merged.Add(topDownTerm, bottomUpTerm)
		denseClipColInPlace(merged, rm.cfg.StateClip)
	}
}

func (rm *ResonanceManifold) stateGradients(
	predictions []*mat.Dense,
	layerErrors []*mat.Dense,
) []*mat.Dense {
	topIndex := len(rm.z) - 1

	for layerIndex := 1; layerIndex <= topIndex; layerIndex++ {
		gradient := rm.workspace.grads[layerIndex]
		gradient.Zero()

		if layerIndex < topIndex {
			if rm.cfg.UsePrecision {
				weightedError := rm.workspace.weightedErr[layerIndex]
				weightedError.MulElem(rm.precisionFor(layerIndex), layerErrors[layerIndex])
				gradient.Add(gradient, weightedError)
			} else {
				gradient.Add(gradient, layerErrors[layerIndex])
			}
		}

		belowSignal := rm.workspace.belowSignal[layerIndex-1]
		denseApplyOneMinusSquareInto(belowSignal, predictions[layerIndex-1])

		if rm.cfg.UsePrecision {
			belowSignal.MulElem(belowSignal, layerErrors[layerIndex-1])
			belowSignal.MulElem(belowSignal, rm.precisionFor(layerIndex-1))
		} else {
			belowSignal.MulElem(belowSignal, layerErrors[layerIndex-1])
		}

		correction := rm.workspace.correction[layerIndex]
		denseMulWeightTransposeInto(correction, rm.W[layerIndex-1], belowSignal)
		gradient.Sub(gradient, correction)

		if layerIndex == topIndex && rm.temporalPriorReady {
			temporalError := rm.workspace.temporalError
			temporalError.Sub(rm.z[topIndex], rm.temporalPrior)

			if rm.cfg.UsePrecision {
				temporalError.MulElem(temporalError, rm.temporalPrecisionVec())
			}

			temporalError.Scale(rm.cfg.TemporalWeight, temporalError)
			gradient.Add(gradient, temporalError)
		}

		if rm.cfg.LatentDecay > 0 {
			decayTerm := rm.workspace.stepBuf[layerIndex]
			decayTerm.Scale(rm.cfg.LatentDecay, rm.z[layerIndex])
			gradient.Add(gradient, decayTerm)
		}

		if rm.cfg.Sparsity > 0 {
			rowCount, _ := rm.z[layerIndex].Dims()

			for rowIndex := range rowCount {
				latentValue := rm.z[layerIndex].At(rowIndex, 0)
				if latentValue > 0 {
					gradient.Set(rowIndex, 0, gradient.At(rowIndex, 0)+rm.cfg.Sparsity)
				}

				if latentValue < 0 {
					gradient.Set(rowIndex, 0, gradient.At(rowIndex, 0)-rm.cfg.Sparsity)
				}
			}
		}

		gradientNorm := denseColNorm(gradient)

		if gradientNorm > rm.cfg.GradClip {
			gradient.Scale(rm.cfg.GradClip/gradientNorm, gradient)
		}
	}

	return rm.workspace.grads
}

func (rm *ResonanceManifold) saveStates() {
	for layerIndex, latent := range rm.z {
		rm.workspace.savedStates[layerIndex].Copy(latent)
	}
}

func (rm *ResonanceManifold) restoreStates() {
	for layerIndex, latent := range rm.z {
		latent.Copy(rm.workspace.savedStates[layerIndex])
	}
}

func (rm *ResonanceManifold) tryStateUpdate(gradients []*mat.Dense, stepSize float64) {
	for layerIndex := 1; layerIndex < len(rm.z); layerIndex++ {
		step := rm.workspace.stepBuf[layerIndex]
		step.Scale(stepSize, gradients[layerIndex])

		nextState := rm.z[layerIndex]
		nextState.Sub(rm.workspace.savedStates[layerIndex], step)
		denseClipColInPlace(nextState, rm.cfg.StateClip)
	}
}

func (rm *ResonanceManifold) updatePrecision(
	layerErrors []*mat.Dense,
	temporalError *mat.Dense,
	targetCol *mat.Dense,
	taskError *mat.Dense,
) error {
	if !rm.cfg.UsePrecision {
		return nil
	}

	beta := rm.cfg.PrecisionBeta

	for layerIndex, layerError := range layerErrors {
		rowCount, _ := rm.errorVar[layerIndex].Dims()

		for rowIndex := range rowCount {
			errorValue := layerError.At(rowIndex, 0)
			variance := rm.errorVar[layerIndex].At(rowIndex, 0)
			variance = (1.0-beta)*variance + beta*(errorValue*errorValue)
			variance = math.Max(rm.cfg.PrecisionEps, variance)
			rm.errorVar[layerIndex].Set(rowIndex, 0, variance)

			if !(variance > 0) {
				return errnie.Err(
					errnie.Validation,
					"resonance: precision variance must be strictly positive",
					nil,
				)
			}

			rawPrecision := 1.0 / variance
			precisionValue := math.Min(rm.cfg.PrecisionMax, math.Max(rm.cfg.PrecisionMin, rawPrecision))
			rm.precision[layerIndex].Set(rowIndex, 0, precisionValue)
		}
	}

	if temporalError != nil {
		rowCount, _ := rm.temporalVar.Dims()

		for rowIndex := range rowCount {
			errorValue := temporalError.At(rowIndex, 0)
			variance := rm.temporalVar.At(rowIndex, 0)
			variance = (1.0-beta)*variance + beta*(errorValue*errorValue)
			variance = math.Max(rm.cfg.PrecisionEps, variance)
			rm.temporalVar.Set(rowIndex, 0, variance)

			if !(variance > 0) {
				return errnie.Err(
					errnie.Validation,
					"resonance: temporal precision variance must be strictly positive",
					nil,
				)
			}

			rawPrecision := 1.0 / variance
			precisionValue := math.Min(rm.cfg.PrecisionMax, math.Max(rm.cfg.PrecisionMin, rawPrecision))
			rm.temporalPrecision.Set(rowIndex, 0, precisionValue)
		}
	}

	if targetCol != nil && taskError != nil && rm.V != nil {
		return rm.updateTaskReliability(targetCol, taskError)
	}

	return nil
}

func (rm *ResonanceManifold) updateTaskReliability(
	targetCol *mat.Dense,
	taskError *mat.Dense,
) error {
	beta := rm.cfg.PrecisionBeta
	rowCount, _ := rm.taskVar.Dims()

	for rowIndex := range rowCount {
		errorValue := taskError.At(rowIndex, 0)
		squaredError := errorValue * errorValue

		if !rm.taskScaleReady[rowIndex] {
			if squaredError > 0 {
				rm.taskVar.Set(rowIndex, 0, squaredError)
				rm.taskScale.Set(rowIndex, 0, math.Log(squaredError))
				rm.taskScaleReady[rowIndex] = true
			}
		} else {
			logScale := rm.taskScale.At(rowIndex, 0)
			scaleVariance := math.Exp(logScale)
			variance := (1.0-beta)*rm.taskVar.At(rowIndex, 0) + beta*squaredError
			varianceFloor := rm.cfg.PrecisionEps * scaleVariance
			variance = math.Max(varianceFloor, variance)
			rm.taskVar.Set(rowIndex, 0, variance)

			if variance > varianceFloor {
				logVariance := math.Log(variance)
				logScale = (1.0-beta)*logScale + beta*logVariance
				rm.taskScale.Set(rowIndex, 0, logScale)
			}

			rawPrecision := math.Exp(rm.taskScale.At(rowIndex, 0)) / variance
			precisionValue := math.Min(
				rm.cfg.PrecisionMax,
				math.Max(rm.cfg.PrecisionMin, rawPrecision),
			)
			rm.taskPrecision.Set(rowIndex, 0, precisionValue)
		}

		rm.updateTaskSkill(rowIndex, targetCol.At(rowIndex, 0), squaredError)
	}

	return nil
}

func (rm *ResonanceManifold) updateTaskSkill(
	rowIndex int,
	targetValue float64,
	modelSquaredError float64,
) {
	if !rm.taskBaselineReady[rowIndex] {
		rm.taskBaselineMean.Set(rowIndex, 0, targetValue)
		rm.taskBaselineReady[rowIndex] = true

		return
	}

	baselineMean := rm.taskBaselineMean.At(rowIndex, 0)
	baselineError := targetValue - baselineMean
	baselineSquaredError := baselineError * baselineError
	rm.taskBaselineMean.Set(
		rowIndex,
		0,
		baselineMean+rm.cfg.PrecisionBeta*baselineError,
	)

	if !rm.taskSkillReady[rowIndex] {
		rm.taskModelLoss.Set(rowIndex, 0, modelSquaredError)
		rm.taskBaselineLoss.Set(rowIndex, 0, baselineSquaredError)
		rm.taskSkillReady[rowIndex] = true
	} else {
		beta := rm.cfg.PrecisionBeta
		modelLoss := (1.0-beta)*rm.taskModelLoss.At(rowIndex, 0) + beta*modelSquaredError
		baselineLoss := (1.0-beta)*rm.taskBaselineLoss.At(rowIndex, 0) +
			beta*baselineSquaredError
		rm.taskModelLoss.Set(rowIndex, 0, modelLoss)
		rm.taskBaselineLoss.Set(rowIndex, 0, baselineLoss)
	}

	modelLoss := rm.taskModelLoss.At(rowIndex, 0)
	baselineLoss := rm.taskBaselineLoss.At(rowIndex, 0)
	lossScale := math.Max(modelLoss, baselineLoss)

	if lossScale == 0 {
		rm.taskSkill.Set(rowIndex, 0, 1.0)

		return
	}

	regularizer := rm.cfg.PrecisionEps * lossScale
	rawSkill := (baselineLoss + regularizer) / (modelLoss + regularizer)
	skill := math.Min(rm.cfg.PrecisionMax, math.Max(rm.cfg.PrecisionMin, rawSkill))
	rm.taskSkill.Set(rowIndex, 0, skill)
}

/*
adoptPace copies the learning-pace family of another config over this one and
leaves the geometry family untouched.

The split exists because the retained weights, error variances and precisions
were all estimated under one state geometry. Re-deriving StateClip,
TopDownInitMix or the inference step counts mid-stream would move the state
space those estimates describe, so a pace change would silently invalidate
learned state instead of merely speeding it up or slowing it down.
*/
func (cfg *ResonanceConfig) adoptPace(pace ResonanceConfig) {
	cfg.LrState = pace.LrState
	cfg.LrGenerative = pace.LrGenerative
	cfg.LrTemporal = pace.LrTemporal
	cfg.LrRecognition = pace.LrRecognition
	cfg.PrecisionBeta = pace.PrecisionBeta

	// TemporalWeight is deliberately absent. It scales the temporal term of
	// the variational objective relative to the generative terms, so it
	// describes the shape of the energy landscape rather than the rate at
	// which the landscape is descended. Moving it with the pace would change
	// what the network is minimizing, and would make the reported prediction
	// energy move whenever the controller retuned alpha with no change in how
	// well the network predicts.
	cfg.LatentDecay = pace.LatentDecay
	cfg.Sparsity = pace.Sparsity
	cfg.WeightDecay = pace.WeightDecay
	cfg.GradClip = pace.GradClip
}

/*
SetAlpha retunes the learning pace of the manifold dynamically.

Only the pace family moves. The state geometry the retained weights and
precisions were fit in stays fixed, so a controller may drive alpha across its
whole range without invalidating what the network has already learned.
*/
func (rm *ResonanceManifold) SetAlpha(alpha float64) error {
	if alpha <= 0 || alpha > 1 || math.IsNaN(alpha) || math.IsInf(alpha, 0) {
		return errors.New("resonance: alpha must be finite and in (0, 1]")
	}

	rm.cfg.adoptPace(AdaptiveResonanceConfig(alpha, rm.arch))

	return nil
}

/*
RolloutRetention reports how much of the initial latent magnitude survives at
each step of a rollout, as a fraction in (0, 1].

The temporal recursion z <- tanh(A * z) is a contraction: tanh is 1-Lipschitz
and every temporal update projects A back inside TemporalNormMax in induced
Euclidean norm, so the trajectory relaxes toward the origin and every task
reading taken along it shrinks with it.
That relaxation is genuine learned dynamics, not an artifact, but it means a
k-step curve is not k equally informative forecasts. Past the point where
retention has decayed, the curve carries the decay envelope rather than any
statement about the market, and a caller that averages or sums across the whole
curve is mostly averaging the envelope.

Retention makes that envelope explicit so callers can weight, truncate, or
simply read only as far as the dynamics still support.
*/
func (rm *ResonanceManifold) RolloutRetention(steps int) []float64 {
	if rm.A == nil || steps < 1 {
		return nil
	}

	topDim := rm.arch[len(rm.arch)-1]
	currentState := mat.DenseCopyOf(rm.z[len(rm.z)-1])
	nextState := mat.NewDense(topDim, 1, nil)

	initialNorm := denseColNorm(currentState)
	retention := make([]float64, steps)

	for step := range steps {
		if step == 0 {
			retention[step] = 1
		} else if initialNorm > 0 {
			retention[step] = denseColNorm(currentState) / initialNorm
		}

		if step+1 < steps {
			nextState.Mul(rm.A, currentState)
			denseApplyTanhInPlace(nextState)
			currentState.Copy(nextState)
		}
	}

	return retention
}

/*
RolloutTaskForecast returns the posterior predictive task distribution at every
supported step. Step zero evaluates the currently settled state because that is
the state the supervised head learned against for the next realized target. Only
later steps advance through the temporal prior.
*/
func (rm *ResonanceManifold) RolloutTaskForecast(steps int) ([]RLSOutput, error) {
	if rm.V == nil || rm.A == nil || rm.targetDim <= 0 || steps < 1 {
		return nil, nil
	}

	topDim := rm.arch[len(rm.arch)-1]
	currentState := mat.DenseCopyOf(rm.z[len(rm.z)-1])
	nextState := mat.NewDense(topDim, 1, nil)
	forecast := make([]RLSOutput, steps*rm.targetDim)

	for step := range steps {
		features := currentState.RawMatrix().Data

		for rowIndex, learner := range rm.taskLearners {
			output, err := learner.Predict(features)

			if err != nil {
				return nil, fmt.Errorf("resonance: task forecast: %w", err)
			}

			forecast[step*rm.targetDim+rowIndex] = output
		}

		if step+1 < steps {
			nextState.Mul(rm.A, currentState)
			denseApplyTanhInPlace(nextState)
			currentState.Copy(nextState)
		}
	}

	return forecast, nil
}

/*
RolloutTaskPrediction projects the top latent state forward k steps into the future
using the temporal prior matrix A, evaluating task head V at each step.
Returns a slice of return predictions [y_t+1, y_t+2, ..., y_t+k].

The latent recursion keeps its tanh, which is what bounds the trajectory and
makes the dynamics stable. The task head does not, for the reason given on
taskPredictionInto: squashing a small-magnitude target only attenuates it.

Callers reading more than the first step should pair this with RolloutRetention,
which reports how much of the latent magnitude still survives at each step and
therefore how much of the curve is forecast rather than relaxation.
*/
func (rm *ResonanceManifold) RolloutTaskPrediction(steps int) []float64 {
	if rm.V == nil || rm.A == nil || rm.targetDim <= 0 || steps < 1 {
		return nil
	}

	topDim := rm.arch[len(rm.arch)-1]
	topLatent := rm.z[len(rm.z)-1]

	// Working state buffers
	currentState := mat.DenseCopyOf(topLatent)
	nextState := mat.NewDense(topDim, 1, nil)
	taskPred := mat.NewDense(rm.targetDim, 1, nil)

	curve := make([]float64, steps*rm.targetDim)

	for step := range steps {
		// The current state predicts the next return.
		taskPred.Mul(rm.V, currentState)
		taskPred.Add(taskPred, rm.taskBias)

		for row := 0; row < rm.targetDim; row++ {
			curve[step*rm.targetDim+row] = taskPred.At(row, 0)
		}

		if step+1 < steps {
			nextState.Mul(rm.A, currentState)
			denseApplyTanhInPlace(nextState)
			currentState.Copy(nextState)
		}
	}

	return curve
}

/*
DynamicHorizon calculates the forward horizon step count supported by relative
task precision, prequential skill against the online-mean baseline, and rollout
retention, updating and returning the current reach.

Reach grows only while the current error is no worse than its retained scale and
the head beats the baseline. Confidence supplies both the proportional reach cap
and the tolerated retention loss, so no fixed decay threshold is assumed.
*/
func (rm *ResonanceManifold) DynamicHorizon(
	confidence float64, currentReach int, maxHorizon int,
) (int, int) {
	precision, hasPrecision := rm.TaskPrecision()
	skill, hasSkill := rm.TaskSkill()
	newReach := currentReach

	switch {
	case !hasPrecision || !hasSkill:
		newReach = 1
	case skill >= 1.0:
		newReach = currentReach + 1
	default:
		newReach = currentReach - 1
	}

	newReach = min(maxHorizon, max(1, newReach))
	horizon := newReach
	relativeReliability := math.Min(1.0, precision)
	supportedConfidence := confidence * relativeReliability

	if capped := int(math.Floor(float64(newReach) * supportedConfidence)); capped < horizon {
		horizon = capped
	}

	if horizon < 1 {
		return 1, newReach
	}

	retention := rm.RolloutRetention(horizon)

	if len(retention) == 0 || retention[0] <= 0 {
		return 1, newReach
	}

	minimumRetention := 1.0 - supportedConfidence

	for step, surviving := range retention {
		if surviving/retention[0] < minimumRetention {
			if step == 0 {
				return 1, newReach
			}

			return step, newReach
		}
	}

	return horizon, newReach
}
