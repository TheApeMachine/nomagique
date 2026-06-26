package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
MedianAbsolute computes the median of absolute values over retained history.
*/
type MedianAbsolute struct {
	artifact *datura.Artifact
}

/*
NewMedianAbsolute returns a median-absolute stage wired from config attributes on the artifact.
*/
func NewMedianAbsolute(artifact *datura.Artifact) *MedianAbsolute {
	return &MedianAbsolute{
		artifact: artifact,
	}
}

func (medianAbsolute *MedianAbsolute) Read(payload []byte) (int, error) {
	state := datura.Acquire("median-absolute-state", datura.APPJSON)

	if _, err := state.Unpack(medianAbsolute.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median-absolute: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median-absolute: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median-absolute: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](medianAbsolute.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median-absolute: input required",
			nil,
		))
	}

	outputKey := datura.Peek[string](medianAbsolute.artifact, "outputKey")

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median-absolute: outputKey required",
			nil,
		))
	}

	var sample float64
	found := false

	for index, input := range inputs {
		if input != configInput {
			continue
		}

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"median-absolute: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		found = true
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median-absolute: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median-absolute: sample is non-finite",
			nil,
		))
	}

	history := datura.Peek[[]float64](medianAbsolute.artifact, "history")
	history = append(history, math.Abs(sample))
	medianAbsolute.artifact.Poke(history, "history")

	value, ok := MedianOf(history)

	if !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median-absolute: unable to compute median",
			nil,
		))
	}

	state.MergeOutput(outputKey, value)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.PackInto(payload)
}

func (medianAbsolute *MedianAbsolute) Write(payload []byte) (int, error) {
	medianAbsolute.artifact.WithPayload(payload)
	return len(payload), nil
}

func (medianAbsolute *MedianAbsolute) Close() error {
	return nil
}

/*
MedianAbsoluteOf returns the median of absolute values.
*/
func MedianAbsoluteOf(values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}

	absolute := make([]float64, len(values))

	for index, value := range values {
		absolute[index] = math.Abs(value)
	}

	return MedianOf(absolute)
}
