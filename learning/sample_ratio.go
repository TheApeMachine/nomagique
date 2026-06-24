package learning

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Calibrator tracks calibration sample ratio from predicted-vs-actual pairs.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Calibrator struct {
	artifact *datura.Artifact
}

/*
SampleRatio returns a calibration stage wired from config attributes on the artifact.
*/
func SampleRatio(artifact *datura.Artifact) *Calibrator {
	return &Calibrator{
		artifact: artifact,
	}
}

func (calibrator *Calibrator) Read(payload []byte) (int, error) {
	state := datura.Acquire("sample-ratio-state", datura.APPJSON)

	if _, err := state.Write(calibrator.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"sample-ratio: state write failed",
			err,
		))
	}

	defer state.Release()

	predicted, actual, err := calibrator.resolvePair(state)

	if err != nil {
		return 0, err
	}

	ratioState := sampleRatioStateFromArtifact(calibrator.artifact)
	derived := ObserveSampleRatio(&ratioState, predicted, actual)
	pokeSampleRatioState(calibrator.artifact, &ratioState, derived)
	state.MergeOutput("value", derived)
	state.MergeOutput("predicted", predicted)
	state.MergeOutput("actual", actual)
	state.Poke("output", "root")
	state.Poke([]string{"value", "predicted", "actual"}, "inputs")
	return state.Read(payload)
}

func (calibrator *Calibrator) resolvePair(state *datura.Artifact) (float64, float64, error) {
	parsedPredicted, parsedActual, err := wirePair(calibrator.artifact, state, "sample-ratio")

	if err != nil {
		return 0, 0, err
	}

	if parsedActual == 0 {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"sample-ratio: actual must be non-zero",
			nil,
		))
	}

	return parsedPredicted, parsedActual, nil
}

func (calibrator *Calibrator) Write(payload []byte) (int, error) {
	calibrator.artifact.WithPayload(payload)
	return len(payload), nil
}

func (calibrator *Calibrator) Close() error {
	return nil
}

/*
SampleRatioState maps predicted and actual pairs into calibration samples.
*/
type SampleRatioState struct {
	Prev      float64
	Min       float64
	Max       float64
	PeakRatio float64
	Ready     bool
}

/*
Observe ingests a predicted and actual pair and returns the calibration sample.
*/
func (state *SampleRatioState) Observe(predicted float64, actual float64) float64 {
	return ObserveSampleRatio(state, predicted, actual)
}

/*
ObserveSamples writes one calibration sample per pair into out.
*/
func (state *SampleRatioState) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	observeSampleRatioSamples(state, predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (state *SampleRatioState) Reset() {
	state.Prev = 0
	state.Min = 0
	state.Max = 0
	state.PeakRatio = 0
	state.Ready = false
}

/*
ObserveSampleRatio maps one predicted and actual pair to a calibration sample.
*/
func ObserveSampleRatio(
	state *SampleRatioState, predicted float64, actual float64,
) float64 {
	if !state.Ready {
		state.Min = actual - predicted
		state.Max = actual - predicted
		state.Prev = predicted
		state.Ready = true

		return bootstrapSampleRatio(predicted, actual)
	}

	return observeSampleRatioReady(state, predicted, actual)
}

func bootstrapSampleRatio(predicted float64, actual float64) float64 {
	ratio := rawSampleRatio(predicted, actual)
	ceiling := maxSampleRatioCeiling(0, absExact(predicted))

	if ratio > ceiling {
		return ceiling
	}

	return ratio
}

/*
observeSampleRatioReady runs the hot sample-ratio path; state must already be Ready.
*/
func observeSampleRatioReady(
	state *SampleRatioState, predicted float64, actual float64,
) float64 {
	residual := actual - predicted

	if residual < state.Min {
		state.Min = residual
	}

	if residual > state.Max {
		state.Max = residual
	}

	span := state.Max - state.Min
	ratio := rawSampleRatio(predicted, actual)
	ceiling := maxSampleRatioCeiling(span, absExact(state.Prev))

	if ratio > ceiling {
		ratio = ceiling
	}

	if ratio > state.PeakRatio {
		state.PeakRatio = ratio
	}

	state.Prev = predicted

	return ratio
}

func rawSampleRatio(predicted float64, actual float64) float64 {
	if actual >= predicted {
		return actual / predicted
	}

	lossRatio := 1 + actual/predicted

	if lossRatio < 0 {
		return 0
	}

	return lossRatio
}

/*
maxSampleRatioCeiling derives the upper calibration bound from observed spread.
*/
func maxSampleRatioCeiling(span float64, reference float64) float64 {
	if span > 0 {
		return 1 + 1/span
	}

	if reference > 0 {
		return 1 + 1/reference
	}

	return 1
}
