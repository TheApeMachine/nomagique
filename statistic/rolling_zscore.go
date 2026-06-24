package statistic

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
RollingZScore normalizes the current sample against its retained series on the stage instance.
*/
type RollingZScore struct {
	artifact *datura.Artifact
	samples  map[string][]struct {
		value float64
		at    time.Time
	}
}

/*
NewRollingZScore returns a rolling z-score stage wired from config attributes on the artifact.
*/
func NewRollingZScore(artifact *datura.Artifact) *RollingZScore {
	return &RollingZScore{
		artifact: artifact,
		samples: map[string][]struct {
			value float64
			at    time.Time
		}{},
	}
}

func (rollingZScore *RollingZScore) Read(payload []byte) (int, error) {
	state := datura.Acquire("rolling-zscore-state", datura.APPJSON)

	if _, err := state.Write(rollingZScore.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](rollingZScore.artifact, "input")
	outputKey := datura.Peek[string](rollingZScore.artifact, "outputKey")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: input required",
			nil,
		))
	}

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: outputKey required",
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
					"rolling-zscore: feature index out of range",
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
			"rolling-zscore: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: sample is non-finite",
			nil,
		))
	}

	seriesKey := datura.Peek[string](rollingZScore.artifact, "seriesKey")

	if seriesKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: seriesKey required",
			nil,
		))
	}

	timestamp := state.Timestamp()

	if timestamp <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: event timestamp required",
			nil,
		))
	}

	observed := time.Unix(0, timestamp)
	history := rollingZScore.samples[seriesKey]

	if len(history) > 0 && observed.Before(history[len(history)-1].at) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: event timestamp must not regress",
			nil,
		))
	}

	history = append(history, struct {
		value float64
		at    time.Time
	}{value: sample, at: observed})

	prior := make([]float64, len(history)-1)

	for index, point := range history[:len(history)-1] {
		prior[index] = point.value
	}

	var score float64

	if len(prior) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: prior sample required",
			nil,
		))
	}

	meanSample := stat.Mean(prior, nil)
	stdSample := stat.StdDev(prior, nil)

	if stdSample <= 0 {
		meanAbsoluteDeviation := 0.0

		for _, priorSample := range prior {
			meanAbsoluteDeviation += math.Abs(priorSample - meanSample)
		}

		meanAbsoluteDeviation /= float64(len(prior))

		delta := sample - meanSample
		scale := meanAbsoluteDeviation

		if scale <= 0 {
			if delta == 0 {
				score = 0
			}

			if delta != 0 {
				score = delta / math.Abs(delta)
			}
		}

		if scale > 0 {
			score = delta / scale
		}
	}

	if stdSample > 0 {
		score = (sample - meanSample) / stdSample
	}

	rollingZScore.samples[seriesKey] = history
	state.MergeOutput(outputKey, score)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (rollingZScore *RollingZScore) Write(payload []byte) (int, error) {
	rollingZScore.artifact.WithPayload(payload)
	return len(payload), nil
}

func (rollingZScore *RollingZScore) Close() error {
	return nil
}
