package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
EMA is a volatility-adaptive exponential moving average stage.
*/
type EMA struct {
	config   *datura.Artifact
	artifact *datura.Artifact
}

/*
NewEMA returns an EMA stage ready to bootstrap from its first observation.
*/
func NewEMA(config *datura.Artifact) *EMA {
	return &EMA{
		config:   config,
		artifact: datura.Acquire("ema", datura.APPJSON),
	}
}

func (ema *EMA) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](ema.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return ema.artifact.Read(p)
	}

	output := datura.Peek[datura.Map[float64]](ema.artifact, "output")

	if output == nil {
		output = datura.Map[float64]{
			"min":   sample,
			"max":   sample,
			"prev":  sample,
			"rate":  0,
			"value": sample,
		}

		ema.artifact.Poke(output, "output")

		return ema.artifact.Read(p)
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		output["prev"] = sample
		ema.artifact.Poke(output, "output")

		return ema.artifact.Read(p)
	}

	delta := math.Abs(sample - output["prev"])
	output["rate"] = delta / span
	output["value"] += output["rate"] * (sample - output["value"])
	output["prev"] = sample

	ema.artifact.Poke(output, "output")

	return ema.artifact.Read(p)
}

func (ema *EMA) Write(p []byte) (int, error) {
	return ema.artifact.Write(p)
}

func (ema *EMA) Close() error {
	return nil
}
