package adaptive

import (
	"context"
	"io"

	"github.com/cinar/indicator/v2/helper"
	"github.com/cinar/indicator/v2/trend"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
EMA is an exponential moving average stage backed by cinar/indicator trend.Ema.
The constructor artifact holds config attributes; Write buffers wire on artifact payload.
*/
type EMA struct {
	artifact *datura.Artifact
	inner    *trend.Ema[float64]
}

/*
NewEMA returns an EMA stage wired from config attributes on the artifact.
*/
func NewEMA(artifact *datura.Artifact) *EMA {
	inner := trend.NewEma[float64]()

	if period := int(datura.Peek[float64](artifact, "period")); period > 0 {
		inner.Period = period
	}

	if smoothing := datura.Peek[float64](artifact, "smoothing"); smoothing > 0 {
		inner.Smoothing = smoothing
	}

	return &EMA{
		artifact: artifact,
		inner:    inner,
	}
}

func (ema *EMA) Read(payload []byte) (int, error) {
	state := datura.Acquire("ema-state", datura.APPJSON)

	if _, err := state.Write(ema.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: state write failed",
			err,
		))
	}

	state.Inspect("adaptive", "ema", "Read()", "p")

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](ema.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: input required",
			nil,
		))
	}

	currentSamples := make([]float64, 0, len(inputs))

	for index, input := range inputs {
		if input != configInput {
			continue
		}

		sample := datura.Peek[float64](state, rootKey, input)

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"ema: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		currentSamples = append(currentSamples, sample)
	}

	if len(currentSamples) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: input not in inputs",
			nil,
		))
	}

	inputChannel := helper.SliceToChanWithContext(context.Background(), currentSamples)
	outputChannel := ema.inner.ComputeWithContext(context.Background(), inputChannel)
	emaValues := helper.ChanToSlice(outputChannel)

	if len(currentSamples) == 0 {
		return 0, io.EOF
	}

	latestEMA := currentSamples[len(currentSamples)-1]

	if len(emaValues) > 0 {
		latestEMA = emaValues[len(emaValues)-1]
	}

	state.MergeOutput("value", latestEMA)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (ema *EMA) Write(payload []byte) (int, error) {
	ema.artifact.WithPayload(payload)
	return len(payload), nil
}

func (ema *EMA) Close() error {
	return nil
}
