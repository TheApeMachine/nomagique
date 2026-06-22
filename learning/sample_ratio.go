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
	state    SampleRatioState
}

/*
SampleRatio returns a calibration stage wired from config attributes on the artifact.
*/
func SampleRatio(artifact *datura.Artifact) *Calibrator {
	artifact.Inspect("learning", "sample-ratio", "SampleRatio()")

	return &Calibrator{
		artifact: artifact,
	}
}

func (calibrator *Calibrator) Write(payload []byte) (int, error) {
	calibrator.artifact.WithPayload(payload)
	return len(payload), nil
}

func (calibrator *Calibrator) Read(payload []byte) (int, error) {
	state := datura.Acquire("sample-ratio-state", datura.APPJSON)
	state.Inspect("learning", "sample-ratio", "Read()", "p")

	if _, err := state.Write(calibrator.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	predicted := datura.Peek[float64](state, "sample")
	actual := datura.Peek[float64](state, "paired")

	if predicted == 0 && actual == 0 {
		features := datura.Peek[[]float64](state, "features")

		if len(features) >= 2 {
			predicted = features[0]
			actual = features[1]
		}
	}

	if predicted == 0 && actual == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"sample-ratio: predicted and actual required",
			nil,
		))
	}

	if actual == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"sample-ratio: actual must be non-zero",
			nil,
		))
	}

	parsedPredicted, parsedActual, err := parsePredictedActual(predicted, []float64{actual})

	if err != nil {
		return 0, err
	}

	derived := ObserveSampleRatio(&calibrator.state, parsedPredicted, parsedActual)
	calibrator.artifact.Poke(derived, "output", "value")
	state.MergeOutput("value", derived)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
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
	calibrator.state.ObserveSamples(predicted, actual, out)
}

/*
Reset clears derived state.
*/
func (calibrator *Calibrator) Reset() error {
	calibrator.state.Reset()
	calibrator.artifact.WithAttributes(datura.Map[any]{})

	return nil
}
