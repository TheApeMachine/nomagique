package adaptive

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Variance tracks an adaptive mean and variance from the observed sample stream.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Variance struct {
	artifact     *datura.Artifact
	bootstrapped bool
}

/*
NewVariance returns a variance stage wired from config attributes on the artifact.
*/
func NewVariance(artifact *datura.Artifact) *Variance {
	artifact.Inspect("adaptive", "variance", "NewVariance()")

	return &Variance{
		artifact: artifact,
	}
}

func (variance *Variance) Write(payload []byte) (int, error) {
	variance.artifact.WithPayload(payload)
	return len(payload), nil
}

func (variance *Variance) Read(payload []byte) (int, error) {
	state := datura.Acquire("variance-state", datura.APPJSON)
	state.Inspect("adaptive", "variance", "Read()", "p")

	if _, err := state.Write(variance.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sampleKey, err := statistic.WireInputKey(variance.artifact, state)

	if err != nil {
		return 0, err
	}

	sample, err := statistic.WireScalar(variance.artifact, state, sampleKey)

	if err != nil {
		return 0, err
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"variance: sample is non-finite",
			nil,
		))
	}

	output := datura.Map[float64]{
		"mean": datura.Peek[float64](variance.artifact, "output", "mean"),
		"var":  datura.Peek[float64](variance.artifact, "output", "var"),
		"prev": datura.Peek[float64](variance.artifact, "output", "prev"),
		"min":  datura.Peek[float64](variance.artifact, "output", "min"),
		"max":  datura.Peek[float64](variance.artifact, "output", "max"),
		"rate": datura.Peek[float64](variance.artifact, "output", "rate"),
	}

	if !variance.bootstrapped {
		output = datura.Map[float64]{
			"mean": sample,
			"var":  0,
			"prev": sample,
			"min":  sample,
			"max":  sample,
			"rate": 0,
		}

		variance.bootstrapped = true
		variance.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"variance: insufficient samples",
			nil,
		))
	}

	output["min"] = math.Min(output["min"], sample)
	output["max"] = math.Max(output["max"], sample)

	span := output["max"] - output["min"]

	if span == 0 {
		output["prev"] = sample
		variance.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"variance: sample span is zero",
			nil,
		))
	}

	delta := math.Abs(sample - output["prev"])
	output["rate"] = delta / span
	deviation := sample - output["mean"]
	output["mean"] += output["rate"] * (sample - output["mean"])
	output["var"] += output["rate"] * (deviation*deviation - output["var"])
	output["prev"] = sample

	if output["var"] <= 0 {
		variance.artifact.Poke(output, "output")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"variance: variance is not positive",
			nil,
		))
	}

	output["value"] = output["var"]

	variance.artifact.Poke(output, "output")
	state.MergeOutput("value", output["value"])
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (variance *Variance) Close() error {
	return nil
}
