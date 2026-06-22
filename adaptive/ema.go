package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
EMA is a volatility-adaptive exponential moving average stage.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type EMA struct {
	artifact *datura.Artifact
}

/*
NewEMA returns an EMA stage wired from config attributes on the artifact.
*/
func NewEMA(artifact *datura.Artifact) *EMA {
	artifact.Inspect("adaptive", "ema", "NewEMA()")

	return &EMA{
		artifact: artifact,
	}
}

func (ema *EMA) Write(payload []byte) (int, error) {
	ema.artifact.WithPayload(payload)
	return len(payload), nil
}

func (ema *EMA) Read(payload []byte) (int, error) {
	state := datura.Acquire("ema-state", datura.APPJSON)
	state.Inspect("adaptive", "ema", "Read()", "p")

	if _, err := state.Write(ema.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"ema: sample is non-finite",
			nil,
		))
	}

	output := datura.Peek[datura.Map[float64]](ema.artifact, "output")

	switch {
	case output == nil:
		output = datura.Map[float64]{
			"min":   sample,
			"max":   sample,
			"prev":  sample,
			"rate":  0,
			"value": sample,
		}
	default:
		output["min"] = math.Min(output["min"], sample)
		output["max"] = math.Max(output["max"], sample)

		span := output["max"] - output["min"]

		if span == 0 {
			output["prev"] = sample
		} else {
			delta := math.Abs(sample - output["prev"])
			output["rate"] = delta / span
			output["value"] += output["rate"] * (sample - output["value"])
			output["prev"] = sample
		}
	}

	ema.artifact.Poke(output, "output")
	state.MergeOutput("value", output["value"])
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (ema *EMA) Close() error {
	return nil
}
