package learning

import (
	"github.com/theapemachine/datura"
)

/*
Calibrator tracks calibration sample ratio from predicted-vs-actual pairs.
*/
type Calibrator struct {
	artifact *datura.Artifact
	state    SampleRatioState
}

/*
SampleRatio returns a calibration dynamic ready from its first observation.
*/
func SampleRatio() *Calibrator {
	return &Calibrator{
		artifact: datura.Acquire("sample-ratio", datura.Artifact_Type_json),
	}
}

func (calibrator *Calibrator) Write(p []byte) (int, error) {
	return calibrator.artifact.Write(p)
}

func (calibrator *Calibrator) Read(p []byte) (int, error) {
	values := float64Batch(calibrator.artifact)

	if len(values) >= 2 {
		predicted, actual, err := parsePredictedActual(values[0], values[1:])

		if err == nil {
			derived := ObserveSampleRatio(&calibrator.state, predicted, actual)
			putFloat64Payload(&calibrator.artifact, "calibrator", derived)
		}
	}

	return calibrator.artifact.Read(p)
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

	return nil
}
