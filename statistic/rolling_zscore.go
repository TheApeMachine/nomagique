package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
RollingZScore normalizes the current sample against its retained series on the stage instance.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type RollingZScore struct {
	artifact *datura.Artifact
	samples  map[string][]Observation
}

/*
NewRollingZScore returns a rolling z-score stage wired from config attributes on the artifact.
*/
func NewRollingZScore(artifact *datura.Artifact) *RollingZScore {
	return &RollingZScore{
		artifact: artifact,
		samples:  map[string][]Observation{},
	}
}

func (rollingZScore *RollingZScore) Write(payload []byte) (int, error) {
	rollingZScore.artifact.WithPayload(payload)
	return len(payload), nil
}

func (rollingZScore *RollingZScore) Read(payload []byte) (int, error) {
	state := datura.Acquire("rolling-zscore-state", datura.APPJSON)

	if _, err := state.Write(rollingZScore.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.Inspect("statistic", "rolling-zscore", "Read()", "p")

	root := datura.Peek[string](state, "root")
	inputs := datura.Peek[[]string](state, "inputs")
	stageKey := rollingZScore.stageKey()
	outputKey := "value"

	if stageKey != "" {
		configuredKey := datura.Peek[string](rollingZScore.artifact, stageKey, "outputKey")

		if configuredKey != "" {
			outputKey = configuredKey
		}
	}

	inputKey := outputKey

	if len(inputs) > 0 {
		inputKey = inputs[0]
	}

	if len(inputs) == 0 && KeyPresent(state, "sample") {
		inputKey = "sample"
	}

	var sample float64

	if root != "" {
		sample = datura.Peek[float64](state, root, inputKey)
	}

	if root == "" {
		sample = datura.Peek[float64](state, inputKey)
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: sample is non-finite",
			nil,
		))
	}

	longHint := 0

	if stageKey != "" {
		longHint = int(datura.Peek[float64](rollingZScore.artifact, stageKey, "longWindow"))
	}

	var score float64
	seriesKey := SeriesKey(rollingZScore.artifact, state, stageKey)
	observed, err := EventTime(rollingZScore.artifact, state)

	if err != nil {
		return 0, err
	}

	history, err := AppendObservation(rollingZScore.samples[seriesKey], sample, observed)

	if err != nil {
		return 0, err
	}

	prior := ObservationValues(history[:len(history)-1])

	if len(prior) == 0 {
		score = 0
	}

	if len(prior) > 0 {
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
	}

	_, longWindow, err := RollingObservationWindows(history, 0, longHint)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rolling-zscore: unable to resolve long window",
			err,
		))
	}

	rollingZScore.samples[seriesKey] = TrimObservations(history, longWindow)
	state.MergeOutput(outputKey, score)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (rollingZScore *RollingZScore) stageKey() string {
	stageKey := datura.Peek[string](rollingZScore.artifact, "stage")

	if stageKey != "" {
		return stageKey
	}

	order := datura.Peek[[]string](rollingZScore.artifact, "order")
	stageIndex := int(datura.Peek[float64](rollingZScore.artifact, "precursor", "stageIndex"))

	if stageIndex <= 0 {
		stageIndex = int(datura.Peek[float64](rollingZScore.artifact, "stageIndex"))
	}

	if stageIndex >= 0 && len(order) > stageIndex {
		return order[stageIndex]
	}

	return ""
}

func (rollingZScore *RollingZScore) Close() error {
	return nil
}
