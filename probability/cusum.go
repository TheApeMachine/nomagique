package probability

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
CUSUM accumulates sequential change evidence from a sample stream.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type CUSUM struct {
	artifact *datura.Artifact
}

/*
NewCUSUM returns a change-detection stage wired from config attributes on the artifact.
*/
func NewCUSUM(artifact *datura.Artifact) *CUSUM {
	return &CUSUM{
		artifact: artifact,
	}
}

func (cusum *CUSUM) Read(payload []byte) (int, error) {
	state := datura.Acquire("cusum-state", datura.APPJSON)

	if _, err := state.Unpack(cusum.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: state write failed",
			err,
		))
	}

	defer state.Release()

	if datura.Peek[float64](state, "reset") != 0 {
		cusum.artifact.Poke(0.0, "output", "target")
		cusum.artifact.Poke(0.0, "output", "positive")
		cusum.artifact.Poke(0.0, "output", "prev")
		cusum.artifact.Poke(0.0, "output", "min")
		cusum.artifact.Poke(0.0, "output", "max")
		cusum.artifact.Poke(0.0, "output", "rate")
		cusum.artifact.Poke(0.0, "output", "count")
		cusum.artifact.Poke(0.0, "output", "value")
		state.MergeOutput("value", 0)
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: reset",
			nil,
		))
	}

	sampleKey := datura.Peek[string](cusum.artifact, "sampleKey")

	if sampleKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: sampleKey required",
			nil,
		))
	}

	wireRoot := datura.Peek[string](state, "root")

	if wireRoot == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: root required",
			nil,
		))
	}

	wireInputs := datura.Peek[[]string](state, "inputs")

	if len(wireInputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: inputs required",
			nil,
		))
	}

	var sample float64
	sampleFound := false

	for wireIndex, wireInput := range wireInputs {
		if wireInput != sampleKey {
			continue
		}

		if wireRoot == "features" {
			features := datura.Peek[[]float64](state, wireRoot)

			if wireIndex >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"cusum: feature index out of range",
					nil,
				))
			}

			sample = features[wireIndex]
		}

		if wireRoot != "features" {
			sample = datura.Peek[float64](state, wireRoot, wireInput)
		}

		sampleFound = true
	}

	if !sampleFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: sample is non-finite",
			nil,
		))
	}

	target := datura.Peek[float64](cusum.artifact, "output", "target")
	positive := datura.Peek[float64](cusum.artifact, "output", "positive")
	prev := datura.Peek[float64](cusum.artifact, "output", "prev")
	minimum := datura.Peek[float64](cusum.artifact, "output", "min")
	maximum := datura.Peek[float64](cusum.artifact, "output", "max")
	rate := datura.Peek[float64](cusum.artifact, "output", "rate")
	count := int(datura.Peek[float64](cusum.artifact, "output", "count"))

	if count == 0 {
		minimum = sample
		maximum = sample
		prev = sample
		target = sample
		positive = 0
		count = 1
	}

	if count > 1 {
		minimum = math.Min(minimum, sample)
		maximum = math.Max(maximum, sample)
		count++
	}

	if count == 1 && sample != minimum {
		minimum = math.Min(minimum, sample)
		maximum = math.Max(maximum, sample)
		count = 2
	}

	span := maximum - minimum

	if span == 0 {
		cusum.artifact.Poke(target, "output", "target")
		cusum.artifact.Poke(positive, "output", "positive")
		cusum.artifact.Poke(prev, "output", "prev")
		cusum.artifact.Poke(minimum, "output", "min")
		cusum.artifact.Poke(maximum, "output", "max")
		cusum.artifact.Poke(rate, "output", "rate")
		cusum.artifact.Poke(float64(count), "output", "count")

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"cusum: sample span is zero",
			nil,
		))
	}

	rate = math.Abs(sample-prev) / span
	drift := rate * span / 2
	excess := sample - target - drift

	if excess > 0 {
		positive += excess
	}

	if excess < 0 {
		positive = 0
	}

	target += rate * (sample - target)
	prev = sample

	cusum.artifact.Poke(target, "output", "target")
	cusum.artifact.Poke(positive, "output", "positive")
	cusum.artifact.Poke(prev, "output", "prev")
	cusum.artifact.Poke(minimum, "output", "min")
	cusum.artifact.Poke(maximum, "output", "max")
	cusum.artifact.Poke(rate, "output", "rate")
	cusum.artifact.Poke(float64(count), "output", "count")
	cusum.artifact.Poke(positive, "output", "value")
	state.MergeOutput("value", positive)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (cusum *CUSUM) Write(payload []byte) (int, error) {
	cusum.artifact.WithPayload(payload)
	return len(payload), nil
}

func (cusum *CUSUM) Close() error {
	return nil
}
