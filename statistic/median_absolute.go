package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
MedianAbsolute measures typical magnitude while ignoring sign.
The constructor artifact holds config; Write buffers inbound wire on its payload.
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

	if _, err := state.Write(medianAbsolute.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.Inspect("statistic", "median-absolute", "Read()", "p")

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median-absolute: sample is non-finite",
			nil,
		))
	}

	history := datura.Peek[[]float64](medianAbsolute.artifact, "history")
	history = append(history, sample)
	medianAbsolute.artifact.Poke(history, "history")

	value, ok := MedianAbsoluteOf(history)

	if !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median-absolute: history is invalid",
			nil,
		))
	}

	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
	return state.Read(payload)
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

	absoluteValues := make([]float64, len(values))

	for index, value := range values {
		absoluteValues[index] = math.Abs(value)
	}

	return MedianOf(absoluteValues)
}
