package adaptive

import (
	"context"

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
	samples  []float64
}

/*
NewEMA returns an EMA stage wired from config attributes on the artifact.
*/
func NewEMA(artifact *datura.Artifact) *EMA {
	artifact.Inspect("adaptive", "ema", "NewEMA()")

	inner := trend.NewEma[float64]()

	if period := int(datura.Peek[float64](artifact, "period")); period > 0 {
		inner.Period = period
	}

	if smoothing := datura.Peek[float64](artifact, "smoothing"); smoothing > 0 {
		inner.Smoothing = smoothing
	}

	return &EMA{
		artifact: artifact,
		samples:  []float64{},
		inner:    inner,
	}
}

func (ema *EMA) Write(p []byte) (int, error) {
	ema.artifact.WithPayload(p)
	return len(p), nil
}

func (ema *EMA) Read(p []byte) (int, error) {
	state := datura.Acquire("ema-state", datura.APPJSON)
	state.Inspect("adaptive", "ema", "Read()", "p")

	if _, err := state.Write(ema.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: state write failed",
			err,
		))
	}

	root := datura.Peek[string](state, "root")
	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		inputs = []string{"sample"}
	}

	for _, input := range inputs {
		sample := datura.Peek[float64](state, root, input)

		if root == "" {
			sample = datura.Peek[float64](state, input)
		}

		ema.samples = append(ema.samples, sample)
	}

	inputChannel := helper.SliceToChanWithContext(context.Background(), ema.samples)
	outputChannel := ema.inner.ComputeWithContext(context.Background(), inputChannel)
	emaValues := helper.ChanToSlice(outputChannel)

	if len(emaValues) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: insufficient samples",
			nil,
		))
	}

	latestEMA := emaValues[len(emaValues)-1]

	state.MergeOutput("value", latestEMA)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})

	return state.Read(p)
}

func (ema *EMA) Close() error {
	return nil
}
