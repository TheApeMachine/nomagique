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
		artifact: datura.Acquire("sample-ratio", datura.APPJSON).RetainStageAttributes(),
	}
}

func (calibrator *Calibrator) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](calibrator.artifact, "output") == nil

	calibrator.artifact.Clear("sample")
	calibrator.artifact.Clear("paired")

	n, err := calibrator.artifact.Write(p)

	if bootstrap {
		calibrator.artifact.Clear("output")
	}

	return n, err
}

func (calibrator *Calibrator) Read(p []byte) (int, error) {
	predicted := datura.Peek[float64](calibrator.artifact, "sample")
	actual := datura.Peek[float64](calibrator.artifact, "paired")

	if predicted == 0 && actual == 0 {
		return calibrator.artifact.Read(p)
	}

	if actual == 0 {
		return calibrator.artifact.Read(p)
	}

	parsedPredicted, parsedActual, err := parsePredictedActual(predicted, []float64{actual})

	if err != nil {
		return calibrator.artifact.Read(p)
	}

	derived := ObserveSampleRatio(&calibrator.state, parsedPredicted, parsedActual)
	calibrator.artifact.Poke(datura.Map[float64]{"value": derived}, "output")

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
	calibrator.artifact.Clear("output")

	return nil
}
