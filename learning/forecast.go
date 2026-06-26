package learning

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Forecaster learns a multiplicative scale from settled predicted-vs-actual outcomes.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Forecaster struct {
	artifact *datura.Artifact
}

/*
Forecast returns a scale-learning stage wired from config attributes on the artifact.
*/
func Forecast(artifact *datura.Artifact) *Forecaster {
	return &Forecaster{
		artifact: artifact,
	}
}

func (forecaster *Forecaster) Read(payload []byte) (int, error) {
	state := datura.Acquire("forecast-state", datura.APPJSON)

	if _, err := state.Unpack(forecaster.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"forecast: state write failed",
			err,
		))
	}

	defer state.Release()

	predicted, actual, err := forecaster.resolvePair(state)

	if err != nil {
		return 0, err
	}

	if predicted == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"forecast: predicted must be non-zero",
			nil,
		))
	}

	residual := actual - predicted
	scale := datura.Peek[float64](forecaster.artifact, "output", "scale")
	trust := datura.Peek[float64](forecaster.artifact, "output", "trust")
	prev := datura.Peek[float64](forecaster.artifact, "output", "prev")
	minimum := datura.Peek[float64](forecaster.artifact, "output", "min")
	maximum := datura.Peek[float64](forecaster.artifact, "output", "max")
	rate := datura.Peek[float64](forecaster.artifact, "output", "rate")
	weightCount := int(datura.Peek[float64](forecaster.artifact, "output", "weightCount"))
	count := int(datura.Peek[float64](forecaster.artifact, "output", "count"))
	derived := scale

	if count == 0 {
		scale = 1
		prev = predicted
		minimum = residual
		maximum = residual
		trust = 1
		weightCount = 1
		count = 1
		derived = scale
	}

	if weightCount > 1 {
		minimum = math.Min(minimum, residual)
		maximum = math.Max(maximum, residual)
		weightCount++
	}

	if weightCount == 1 && residual != minimum {
		minimum = math.Min(minimum, residual)
		maximum = math.Max(maximum, residual)
		weightCount = 2
	}

	span := maximum - minimum

	if count > 1 || weightCount > 1 {
		if span == 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"forecast: residual span is zero",
				nil,
			))
		}

		surprise := absExact(residual) / span
		rate = surprise
		targetTrust := 1 - surprise

		if targetTrust < 0 {
			targetTrust = 0
		}

		trust += surprise * (targetTrust - trust)
		prev = predicted
		learningRate := surprise * (1 - trust)
		targetScale := actual / predicted
		scale += learningRate * (targetScale - scale)
		count++
		derived = scale
	}

	forecaster.artifact.Poke(scale, "output", "scale")
	forecaster.artifact.Poke(trust, "output", "trust")
	forecaster.artifact.Poke(prev, "output", "prev")
	forecaster.artifact.Poke(minimum, "output", "min")
	forecaster.artifact.Poke(maximum, "output", "max")
	forecaster.artifact.Poke(rate, "output", "rate")
	forecaster.artifact.Poke(float64(weightCount), "output", "weightCount")
	forecaster.artifact.Poke(float64(count), "output", "count")
	forecaster.artifact.Poke(derived, "output", "value")
	state.MergeOutput("value", derived)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (forecaster *Forecaster) resolvePair(state *datura.Artifact) (float64, float64, error) {
	parsedPredicted, parsedActual, err := wirePair(forecaster.artifact, state, "forecast")

	if err != nil {
		return 0, 0, err
	}

	if parsedActual == 0 {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"forecast: actual must be non-zero",
			nil,
		))
	}

	return parsedPredicted, parsedActual, nil
}

func (forecaster *Forecaster) Write(payload []byte) (int, error) {
	forecaster.artifact.WithPayload(payload)
	return len(payload), nil
}

func (forecaster *Forecaster) Close() error {
	return nil
}
