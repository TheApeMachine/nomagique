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

		return 0, err
	}

	state.Inspect("learning", "sample-ratio", "Read()", "p")
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
ObserveSamples runs the exact batch kernel over pairs into out.
*/
func (calibrator *Calibrator) ObserveSamples(
	predicted []float64, actual []float64, out []float64,
) {
	ratioState := sampleRatioStateFromArtifact(calibrator.artifact)
	observeSampleRatioSamples(&ratioState, predicted, actual, out)

	if len(out) > 0 {
		pokeSampleRatioState(calibrator.artifact, &ratioState, out[len(out)-1])
	}
}

/*
Reset clears derived state.
*/
func (calibrator *Calibrator) Reset() error {
	calibrator.artifact.Poke(0.0, "output", "prev")
	calibrator.artifact.Poke(0.0, "output", "min")
	calibrator.artifact.Poke(0.0, "output", "max")
	calibrator.artifact.Poke(0.0, "output", "peakRatio")
	calibrator.artifact.Poke(0.0, "output", "ready")
	calibrator.artifact.Poke(0.0, "output", "value")

	return nil
}
