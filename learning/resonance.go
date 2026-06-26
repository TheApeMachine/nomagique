package learning

import (
	"errors"
	"math"
	"math/rand"

	"github.com/theapemachine/datura"
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
Optional config artifact attributes under "resonance" override derived values.
*/
func AdaptiveResonanceConfig(
	alpha float64, arch []int, config *datura.Artifact,
) ResonanceConfig {
	depth := len(arch)
	depthFloat := float64(depth)

	topDownInitMix := (depthFloat - 1.0) / depthFloat
	temporalWeight := alpha / (alpha + 1.0/depthFloat)
	earlyStopPatience := int(math.Max(1, math.Ceil(math.Sqrt(depthFloat))))
	gradClip := alpha * depthFloat
	stateClip := depthFloat / alpha

	if config != nil {
		if override := datura.Peek[float64](config, "resonance", "temporalWeight"); override > 0 {
			temporalWeight = override
		}

		if override := datura.Peek[float64](config, "resonance", "topDownInitMix"); override > 0 {
			topDownInitMix = override
		}

		if override := int(datura.Peek[float64](config, "resonance", "earlyStopPatience")); override > 0 {
			earlyStopPatience = override
		}

		if override := datura.Peek[float64](config, "resonance", "gradClip"); override > 0 {
			gradClip = override
		}

		if override := datura.Peek[float64](config, "resonance", "stateClip"); override > 0 {
			stateClip = override
		}
	}

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

		TemporalWeight: temporalWeight,
		TopDownInitMix: topDownInitMix,

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
	artifact              *datura.Artifact
	cfg                   ResonanceConfig
	arch                  []int
	targetDim             int
	W                     []*mat.Dense
	R                     []*mat.Dense
	A                     *mat.Dense
	V                     *mat.Dense
	z                     []*mat.Dense
	prevTop               *mat.Dense
	errorVar              []*mat.Dense
	precision             []*mat.Dense
	temporalVar           *mat.Dense
	temporalPrecision     *mat.Dense
	taskVar               *mat.Dense
	taskPrecision         *mat.Dense
	workspace             *resonanceWorkspace
	streamLearn           bool
	streamAdvanceTemporal bool
	output                float64
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

	cfg := AdaptiveResonanceConfig(alpha, arch, nil)
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
	var taskVar *mat.Dense
	var taskPrecision *mat.Dense

	if targetDim > 0 {
		scaleV := math.Sqrt(2.0 / float64(topDim+targetDim))
		dataV := make([]float64, targetDim*topDim)

		for index := range dataV {
			dataV[index] = rng.NormFloat64() * scaleV
		}

		taskWeights = mat.NewDense(targetDim, topDim, dataV)
		taskVar = mat.NewDense(targetDim, 1, nil)
		taskPrecision = mat.NewDense(targetDim, 1, nil)

		for rowIndex := 0; rowIndex < targetDim; rowIndex++ {
			taskVar.Set(rowIndex, 0, 1.0)
			taskPrecision.Set(rowIndex, 0, 1.0)
		}
	}

	latents := make([]*mat.Dense, len(arch))

	for layerIndex, layerDim := range arch {
		latents[layerIndex] = mat.NewDense(layerDim, 1, nil)
	}

	temporalVar := mat.NewDense(topDim, 1, nil)
	temporalPrecision := mat.NewDense(topDim, 1, nil)

	for rowIndex := 0; rowIndex < topDim; rowIndex++ {
		temporalVar.Set(rowIndex, 0, 1.0)
		temporalPrecision.Set(rowIndex, 0, 1.0)
	}

	return &ResonanceManifold{
		artifact:              datura.Acquire("resonance", datura.APPJSON),
		cfg:                   cfg,
		arch:                  arch,
		targetDim:             targetDim,
		W:                     weights,
		R:                     recognition,
		A:                     temporalWeights,
		V:                     taskWeights,
		z:                     latents,
		errorVar:              errorVar,
		precision:             precision,
		temporalVar:           temporalVar,
		temporalPrecision:     temporalPrecision,
		taskVar:               taskVar,
		taskPrecision:         taskPrecision,
		workspace:             newResonanceWorkspace(arch, targetDim),
		streamLearn:           true,
		streamAdvanceTemporal: true,
	}, nil
}

func (rm *ResonanceManifold) ResetState(resetPrecision bool) {
	for _, latent := range rm.z {
		rowCount, _ := latent.Dims()
		for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
			latent.Set(rowIndex, 0, 0.0)
		}
	}
	rm.prevTop = nil

	if resetPrecision {
		for layerIndex := 0; layerIndex < len(rm.W); layerIndex++ {
			rowCount, _ := rm.errorVar[layerIndex].Dims()
			for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
				rm.errorVar[layerIndex].Set(rowIndex, 0, 1.0)
				rm.precision[layerIndex].Set(rowIndex, 0, 1.0)
			}
		}
		topDim := rm.arch[len(rm.arch)-1]
		for rowIndex := 0; rowIndex < topDim; rowIndex++ {
			rm.temporalVar.Set(rowIndex, 0, 1.0)
			rm.temporalPrecision.Set(rowIndex, 0, 1.0)
		}
		if rm.targetDim > 0 {
			for rowIndex := 0; rowIndex < rm.targetDim; rowIndex++ {
				rm.taskVar.Set(rowIndex, 0, 1.0)
				rm.taskPrecision.Set(rowIndex, 0, 1.0)
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

func (rm *ResonanceManifold) Read(payload []byte) (int, error) {
	state := datura.Acquire("resonance-state", datura.APPJSON)

	if _, err := state.Unpack(rm.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"resonance: state write failed",
			err,
		))
	}

	values := datura.Peek[[]float64](state, "batch")

	if len(values) < rm.arch[0] {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"resonance: batch shorter than input dimension",
			nil,
		))
	}

	input := values[:rm.arch[0]]
	target := []float64(nil)

	if rm.targetDim > 0 && len(values) >= rm.arch[0]+rm.targetDim {
		target = values[rm.arch[0] : rm.arch[0]+rm.targetDim]
	}

	reconstruction, err := rm.SettleFromBatch(input, target)

	if err != nil {
		return 0, err
	}

	latent := rm.LatentState()

	state.MergeOutput("value", reconstruction)
	state.MergeOutput("latent", latent)
	state.Poke("output", "root")
	state.Poke([]string{"value", "latent"}, "inputs")
	return state.PackInto(payload)
}

func (rm *ResonanceManifold) Write(payload []byte) (int, error) {
	if payloadHasReset(payload) {
		rm.ResetState(false)

		return len(payload), nil
	}

	rm.artifact.WithPayload(payload)
	return len(payload), nil
}

func (rm *ResonanceManifold) Close() error {
	return nil
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

	err := rm.Settle(input, advanceTemporal)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"resonance: settle failed",
			err,
		))
	}

	if learn {
		rm.Learn(target)
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

	xCol := rm.workspace.xCol
	for rowIndex, value := range input {
		xCol.Set(rowIndex, 0, value)
	}

	rm.initializeLatents(xCol)

	trace := []float64{rm.Energy()}

	for step := 0; step < rm.cfg.MaxInferenceSteps; step++ {
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

			if !rm.cfg.MonotoneStateSteps || newEnergy <= oldEnergy+1e-12 {
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
		} else {
			trace = append(trace, rm.Energy())
		}

		deltaEnergy := math.Abs(trace[len(trace)-2] - trace[len(trace)-1])
		relativeDelta := deltaEnergy / (math.Abs(trace[len(trace)-2]) + 1e-12)

		if step+1 >= rm.cfg.MinInferenceSteps && relativeDelta < rm.cfg.EarlyStopTol {
			break
		}
	}

	if advanceTemporal {
		rm.advanceTemporalState()
	}

	return nil
}

func (rm *ResonanceManifold) Learn(target []float64) {
	predictions, layerErrors := rm.predictAdjacentLayers()
	topIndex := len(rm.z) - 1

	var targetCol *mat.Dense
	if len(target) == rm.targetDim && rm.targetDim > 0 {
		targetCol = rm.workspace.yCol
		for rowIndex, value := range target {
			targetCol.Set(rowIndex, 0, value)
		}
	}

	for layerIndex, weightMatrix := range rm.W {
		localSignal := rm.workspace.localSignal[layerIndex]
		localSignal.Apply(func(rowIndex, colIndex int, value float64) float64 {
			return 1.0 - value*value
		}, predictions[layerIndex])

		precision := rm.precisionFor(layerIndex)
		localSignal.MulElem(localSignal, layerErrors[layerIndex])
		localSignal.MulElem(localSignal, precision)

		update := rm.workspace.weightUpdate[layerIndex]
		update.Outer(1.0, localSignal.ColView(0), rm.z[layerIndex+1].ColView(0))

		norm := mat.Norm(update, 2)
		if norm > rm.cfg.GradClip {
			update.Scale(rm.cfg.GradClip/(norm+1e-12), update)
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
		proposal.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, proposal)

		recError := rm.workspace.recError[layerIndex]
		recError.Sub(rm.z[layerIndex+1], proposal)

		recSignal := rm.workspace.recSignal[layerIndex]
		recSignal.Apply(func(rowIndex, colIndex int, value float64) float64 { return 1.0 - value*value }, proposal)
		recSignal.MulElem(recSignal, recError)

		update := rm.workspace.recUpdate[layerIndex]
		update.Outer(1.0, recSignal.ColView(0), rm.z[layerIndex].ColView(0))

		norm := mat.Norm(update, 2)
		if norm > rm.cfg.GradClip {
			update.Scale(rm.cfg.GradClip/(norm+1e-12), update)
		}

		update.Scale(rm.cfg.LrRecognition, update)
		recognitionMatrix.Add(recognitionMatrix, update)

		if rm.cfg.WeightDecay > 0 {
			recognitionMatrix.Scale(1.0-rm.cfg.LrRecognition*rm.cfg.WeightDecay, recognitionMatrix)
		}
	}

	if targetCol != nil && rm.V != nil {
		taskPred := rm.workspace.taskPred
		taskPred.Mul(rm.V, rm.z[topIndex])
		taskPred.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, taskPred)

		taskError := rm.workspace.taskError
		taskError.Sub(targetCol, taskPred)

		taskSignal := rm.workspace.taskSignal
		taskSignal.Apply(func(rowIndex, colIndex int, value float64) float64 { return 1.0 - value*value }, taskPred)

		precision := rm.taskPrecisionVec()
		taskSignal.MulElem(taskSignal, taskError)
		taskSignal.MulElem(taskSignal, precision)

		update := rm.workspace.taskUpdate
		update.Outer(1.0, taskSignal.ColView(0), rm.z[topIndex].ColView(0))

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

	if rm.prevTop != nil {
		topPrior := rm.workspace.topPrior
		topPrior.Mul(rm.A, rm.prevTop)
		topPrior.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, topPrior)

		temporalError := rm.workspace.temporalError
		temporalError.Sub(rm.z[topIndex], topPrior)

		temporalSignal := rm.workspace.temporalSignal
		temporalSignal.Apply(func(rowIndex, colIndex int, value float64) float64 {
			return 1.0 - value*value
		}, topPrior)

		precision := rm.temporalPrecisionVec()
		temporalSignal.MulElem(temporalSignal, temporalError)
		temporalSignal.MulElem(temporalSignal, precision)
		temporalSignal.Scale(rm.cfg.TemporalWeight, temporalSignal)

		update := rm.workspace.temporalUpdate
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

	rm.updatePrecision(layerErrors, targetCol)
	rm.advanceTemporalState()
}

func (rm *ResonanceManifold) Energy() float64 {
	_, layerErrors := rm.predictAdjacentLayers()
	energy := 0.0

	for layerIndex, layerError := range layerErrors {
		if rm.cfg.UsePrecision {
			weightedError := rm.workspace.weightedErr[layerIndex]
			weightedError.MulElem(rm.precisionFor(layerIndex), layerError)
			energy += 0.5 * mat.Dot(weightedError.ColView(0), layerError.ColView(0))
			continue
		}

		energy += 0.5 * mat.Dot(layerError.ColView(0), layerError.ColView(0))
	}

	if rm.prevTop != nil {
		topPrior := rm.workspace.topPrior
		topPrior.Mul(rm.A, rm.prevTop)
		topPrior.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, topPrior)

		temporalError := rm.workspace.temporalError
		temporalError.Sub(rm.z[len(rm.z)-1], topPrior)

		if rm.cfg.UsePrecision {
			weightedError := rm.workspace.temporalWeightedErr
			weightedError.MulElem(rm.temporalPrecisionVec(), temporalError)
			energy += 0.5 * rm.cfg.TemporalWeight * mat.Dot(weightedError.ColView(0), temporalError.ColView(0))
		} else {
			energy += 0.5 * rm.cfg.TemporalWeight * mat.Dot(temporalError.ColView(0), temporalError.ColView(0))
		}
	}

	if rm.cfg.LatentDecay > 0 {
		for layerIndex := 1; layerIndex < len(rm.z); layerIndex++ {
			norm := mat.Norm(rm.z[layerIndex], 2)
			energy += 0.5 * rm.cfg.LatentDecay * norm * norm
		}
	}

	if rm.cfg.Sparsity > 0 {
		for layerIndex := 1; layerIndex < len(rm.z); layerIndex++ {
			rowCount, _ := rm.z[layerIndex].Dims()
			for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
				energy += rm.cfg.Sparsity * math.Abs(rm.z[layerIndex].At(rowIndex, 0))
			}
		}
	}

	return energy
}

func (rm *ResonanceManifold) ReconstructionError() float64 {
	reconstruction := rm.workspace.reconPred
	reconstruction.Mul(rm.W[0], rm.z[1])
	reconstruction.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, reconstruction)

	diff := rm.workspace.reconDiff
	diff.Sub(rm.z[0], reconstruction)

	return mat.Norm(diff, 2)
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
*/
type ResonanceLayerWire struct {
	State      []float64
	Prediction []float64
	ErrorNorm  float64
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

	for layerIndex := range rm.z {
		stateMatrix := rm.z[layerIndex]
		rowCount, _ := stateMatrix.Dims()
		state := make([]float64, rowCount)
		prediction := make([]float64, rowCount)

		for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
			state[rowIndex] = stateMatrix.At(rowIndex, 0)

			if layerIndex < len(predictions) {
				prediction[rowIndex] = predictions[layerIndex].At(rowIndex, 0)
			}
		}

		errorNorm := 0.0

		if layerIndex < len(layerErrors) {
			errorNorm = mat.Norm(layerErrors[layerIndex], 2)
		}

		layers[layerIndex] = ResonanceLayerWire{
			State:      state,
			Prediction: prediction,
			ErrorNorm:  errorNorm,
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

func (rm *ResonanceManifold) taskPrecisionVec() *mat.Dense {
	return rm.taskPrecision
}

func (rm *ResonanceManifold) predictAdjacentLayers() ([]*mat.Dense, []*mat.Dense) {
	for layerIndex := 0; layerIndex < len(rm.W); layerIndex++ {
		prediction := rm.workspace.predictions[layerIndex]
		prediction.Mul(rm.W[layerIndex], rm.z[layerIndex+1])
		prediction.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, prediction)

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
		proposal.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, proposal)
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
	topPrior.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, topPrior)

	topDown := rm.workspace.topDown
	topDown[len(topDown)-1].Copy(topPrior)

	for layerIndex := len(rm.W) - 1; layerIndex > 0; layerIndex-- {
		proposal := topDown[layerIndex]
		proposal.Mul(rm.W[layerIndex], topDown[layerIndex+1])
		proposal.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, proposal)
	}

	initMix := rm.cfg.TopDownInitMix
	for layerIndex := 1; layerIndex < len(rm.z); layerIndex++ {
		topDownTerm := rm.workspace.mergeTD[layerIndex]
		topDownTerm.Scale(initMix, topDown[layerIndex])

		bottomUpTerm := rm.workspace.mergeBU[layerIndex]
		bottomUpTerm.Scale(1.0-initMix, bottomUp[layerIndex])

		merged := rm.z[layerIndex]
		merged.Add(topDownTerm, bottomUpTerm)
		merged.Apply(func(rowIndex, colIndex int, value float64) float64 {
			return math.Min(rm.cfg.StateClip, math.Max(-rm.cfg.StateClip, value))
		}, merged)
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
		belowSignal.Apply(func(rowIndex, colIndex int, value float64) float64 {
			return 1.0 - value*value
		}, predictions[layerIndex-1])

		if rm.cfg.UsePrecision {
			belowSignal.MulElem(belowSignal, layerErrors[layerIndex-1])
			belowSignal.MulElem(belowSignal, rm.precisionFor(layerIndex-1))
		} else {
			belowSignal.MulElem(belowSignal, layerErrors[layerIndex-1])
		}

		correction := rm.workspace.correction[layerIndex]
		correction.Mul(rm.W[layerIndex-1].T(), belowSignal)
		gradient.Sub(gradient, correction)

		if layerIndex == topIndex && rm.prevTop != nil {
			topPrior := rm.workspace.topPrior
			topPrior.Mul(rm.A, rm.prevTop)
			topPrior.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, topPrior)

			temporalError := rm.workspace.temporalError
			temporalError.Sub(rm.z[topIndex], topPrior)

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
			for rowIndex := 0; rowIndex < rowCount; rowIndex++ {
				latentValue := rm.z[layerIndex].At(rowIndex, 0)
				if latentValue > 0 {
					gradient.Set(rowIndex, 0, gradient.At(rowIndex, 0)+rm.cfg.Sparsity)
				}
				if latentValue < 0 {
					gradient.Set(rowIndex, 0, gradient.At(rowIndex, 0)-rm.cfg.Sparsity)
				}
			}
		}

		gradientNorm := mat.Norm(gradient, 2)
		if gradientNorm > rm.cfg.GradClip {
			gradient.Scale(rm.cfg.GradClip/(gradientNorm+1e-12), gradient)
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
		nextState.Apply(func(rowIndex, colIndex int, value float64) float64 {
			return math.Min(rm.cfg.StateClip, math.Max(-rm.cfg.StateClip, value))
		}, nextState)
	}
}

func (rm *ResonanceManifold) updatePrecision(layerErrors []*mat.Dense, targetCol *mat.Dense) {
	if !rm.cfg.UsePrecision {
		return
	}

	beta := rm.cfg.PrecisionBeta
	topIndex := len(rm.z) - 1

	for layerIndex, layerError := range layerErrors {
		rowCount, _ := rm.errorVar[layerIndex].Dims()

		for rowIndex := range rowCount {
			errorValue := layerError.At(rowIndex, 0)
			variance := rm.errorVar[layerIndex].At(rowIndex, 0)
			variance = (1.0-beta)*variance + beta*(errorValue*errorValue)
			rm.errorVar[layerIndex].Set(rowIndex, 0, variance)

			rawPrecision := 1.0 / (variance + rm.cfg.PrecisionEps)
			precisionValue := math.Min(rm.cfg.PrecisionMax, math.Max(rm.cfg.PrecisionMin, rawPrecision))
			rm.precision[layerIndex].Set(rowIndex, 0, precisionValue)
		}
	}

	if rm.prevTop != nil {
		topPrior := rm.workspace.topPrior
		topPrior.Mul(rm.A, rm.prevTop)
		topPrior.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, topPrior)

		temporalError := rm.workspace.temporalError
		temporalError.Sub(rm.z[topIndex], topPrior)

		rowCount, _ := rm.temporalVar.Dims()

		for rowIndex := range rowCount {
			errorValue := temporalError.At(rowIndex, 0)
			variance := rm.temporalVar.At(rowIndex, 0)
			variance = (1.0-beta)*variance + beta*(errorValue*errorValue)
			rm.temporalVar.Set(rowIndex, 0, variance)

			rawPrecision := 1.0 / (variance + rm.cfg.PrecisionEps)
			precisionValue := math.Min(rm.cfg.PrecisionMax, math.Max(rm.cfg.PrecisionMin, rawPrecision))
			rm.temporalPrecision.Set(rowIndex, 0, precisionValue)
		}
	}

	if targetCol != nil && rm.V != nil {
		taskPred := rm.workspace.taskPred
		taskPred.Mul(rm.V, rm.z[topIndex])
		taskPred.Apply(func(rowIndex, colIndex int, value float64) float64 { return math.Tanh(value) }, taskPred)

		taskError := rm.workspace.taskError
		taskError.Sub(targetCol, taskPred)

		rowCount, _ := rm.taskVar.Dims()

		for rowIndex := range rowCount {
			errorValue := taskError.At(rowIndex, 0)
			variance := rm.taskVar.At(rowIndex, 0)
			variance = (1.0-beta)*variance + beta*(errorValue*errorValue)
			rm.taskVar.Set(rowIndex, 0, variance)

			rawPrecision := 1.0 / (variance + rm.cfg.PrecisionEps)
			precisionValue := math.Min(rm.cfg.PrecisionMax, math.Max(rm.cfg.PrecisionMin, rawPrecision))
			rm.taskPrecision.Set(rowIndex, 0, precisionValue)
		}
	}
}
