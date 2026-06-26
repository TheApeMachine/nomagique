package learning

import (
	"math"

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

	if _, err := state.Unpack(calibrator.artifact.DecryptPayload()); err != nil {
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

	residual := actual - predicted
	prev := datura.Peek[float64](calibrator.artifact, "output", "prev")
	minimum := datura.Peek[float64](calibrator.artifact, "output", "min")
	maximum := datura.Peek[float64](calibrator.artifact, "output", "max")
	peakRatio := datura.Peek[float64](calibrator.artifact, "output", "peakRatio")
	count := int(datura.Peek[float64](calibrator.artifact, "output", "count"))

	if count == 0 {
		minimum = residual
		maximum = residual
		prev = predicted
		count = 1
	}

	if count > 1 {
		minimum = math.Min(minimum, residual)
		maximum = math.Max(maximum, residual)
		count++
	}

	if count == 1 && residual != minimum {
		minimum = math.Min(minimum, residual)
		maximum = math.Max(maximum, residual)
		count = 2
	}

	span := maximum - minimum
	ratio := actual / predicted

	if actual < predicted {
		lossRatio := 1 + actual/predicted

		if lossRatio < 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"sample-ratio: loss ratio is negative",
				nil,
			))
		}

		ratio = lossRatio
	}

	ceiling := 1.0

	if span > 0 {
		ceiling = 1 + 1/span
	}

	if span == 0 && absExact(prev) > 0 {
		ceiling = 1 + 1/absExact(prev)
	}

	if ratio > ceiling {
		ratio = ceiling
	}

	if ratio > peakRatio {
		peakRatio = ratio
	}

	prev = predicted

	calibrator.artifact.Poke(prev, "output", "prev")
	calibrator.artifact.Poke(minimum, "output", "min")
	calibrator.artifact.Poke(maximum, "output", "max")
	calibrator.artifact.Poke(peakRatio, "output", "peakRatio")
	calibrator.artifact.Poke(float64(count), "output", "count")
	calibrator.artifact.Poke(ratio, "output", "value")
	state.MergeOutput("value", ratio)
	state.MergeOutput("predicted", predicted)
	state.MergeOutput("actual", actual)
	state.Poke("output", "root")
	state.Poke([]string{"value", "predicted", "actual"}, "inputs")

	return state.PackInto(payload)
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
