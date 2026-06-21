package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
EMA is a volatility-adaptive exponential moving average stage.
*/
type EMA struct {
	artifact *datura.Artifact
	bytes    []byte
}

/*
NewEMA returns an EMA stage ready to bootstrap from its first observation.
*/
func NewEMA(artifact *datura.Artifact) *EMA {
	return &EMA{
		artifact: artifact,
	}
}

func (ema *EMA) Read(payload []byte) (int, error) {
	state := datura.Acquire("ema-state", datura.APPJSON)

	if _, err := state.Write(ema.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return state.Read(payload)
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

	ema.artifact.Merge("output", output)

	result := datura.Acquire("ema-output", datura.APPJSON)
	body := state.DecryptPayload()

	if len(body) == 0 {
		body = []byte("{}")
	}

	result.WithPayload(body)
	result.MergeOutput("value", output["value"])

	return result.Read(payload)
}

func (ema *EMA) Write(payload []byte) (int, error) {
	ema.bytes = append(ema.bytes[:0], payload...)

	return len(payload), nil
}

func (ema *EMA) Close() error {
	return nil
}
