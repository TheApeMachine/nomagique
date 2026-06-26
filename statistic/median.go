package statistic

import (
	"math"
	"sort"
	"strconv"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Median computes the sample median over retained history or panel peers.
*/
type Median struct {
	artifact *datura.Artifact
}

/*
NewMedian returns a median stage wired from config attributes on the artifact.
*/
func NewMedian(artifact *datura.Artifact) *Median {
	return &Median{
		artifact: artifact,
	}
}

func (median *Median) Read(payload []byte) (int, error) {
	state := datura.Acquire("median-state", datura.APPJSON)

	if _, err := state.Unpack(median.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: state write failed",
			err,
		))
	}

	rawPeers := datura.Peek[map[string]any](state, "peers")

	if len(rawPeers) > 0 {
		memberField := datura.Peek[string](median.artifact, "memberKey")

		if memberField == "" {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"median: memberKey required",
				nil,
			))
		}

		rootKey := datura.Peek[string](state, "root")

		if rootKey == "" {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"median: root required",
				nil,
			))
		}

		inputs := datura.Peek[[]string](state, "inputs")

		if len(inputs) == 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"median: inputs required",
				nil,
			))
		}

		var member float64
		memberFound := false

		for index, input := range inputs {
			if input != memberField {
				continue
			}

			if rootKey == "features" {
				features := datura.Peek[[]float64](state, rootKey)

				if index >= len(features) {
					return 0, errnie.Error(errnie.Err(
						errnie.Validation,
						"median: feature index out of range",
						nil,
					))
				}

				member = features[index]
			}

			if rootKey != "features" {
				member = datura.Peek[float64](state, rootKey, input)
			}

			memberFound = true
		}

		if !memberFound {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"median: member not in inputs",
				nil,
			))
		}

		memberLabel := strconv.FormatFloat(member, 'g', -1, 64)
		peerSamples := make([]float64, 0, len(rawPeers))

		for peerKey, rawSample := range rawPeers {
			if peerKey == memberLabel {
				continue
			}

			peerSample, ok := rawSample.(float64)

			if !ok {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"median: peer sample is invalid",
					nil,
				))
			}

			peerSamples = append(peerSamples, peerSample)
		}

		if len(peerSamples) == 0 {
			if len(rawPeers) == 1 {
				soleSample, ok := rawPeers[memberLabel].(float64)

				if !ok {
					return 0, errnie.Error(errnie.Err(
						errnie.Validation,
						"median: sole peer sample is invalid",
						nil,
					))
				}

				state.MergeOutput("value", soleSample)
				state.Poke("output", "root")
				state.Poke([]string{"value"}, "inputs")

				return state.PackInto(payload)
			}

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"median: no peer samples for member",
				nil,
			))
		}

		for _, peerSample := range peerSamples {
			if math.IsNaN(peerSample) || math.IsInf(peerSample, 0) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"median: peer samples are invalid",
					nil,
				))
			}
		}

		sorted := append([]float64(nil), peerSamples...)
		sort.Float64s(sorted)
		middle := len(sorted) / 2
		value := sorted[middle]

		if len(sorted)%2 == 0 {
			value = (sorted[middle-1] + sorted[middle]) / 2
		}

		state.MergeOutput("value", value)
		state.Poke("output", "root")
		state.Poke([]string{"value"}, "inputs")

		return state.PackInto(payload)
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](median.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: input required",
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
					"median: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, 0, input)
		}

		found = true
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: input not in inputs",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: sample is non-finite",
			nil,
		))
	}

	history := datura.Peek[[]float64](median.artifact, "history")
	history = append(history, sample)
	median.artifact.Poke(history, "history")

	for _, historyValue := range history {
		if math.IsNaN(historyValue) || math.IsInf(historyValue, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"median: history is invalid",
				nil,
			))
		}
	}

	sorted := append([]float64(nil), history...)
	sort.Float64s(sorted)
	middle := len(sorted) / 2
	value := sorted[middle]

	if len(sorted)%2 == 0 {
		value = (sorted[middle-1] + sorted[middle]) / 2
	}

	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (median *Median) Write(payload []byte) (int, error) {
	median.artifact.WithPayload(payload)
	return len(payload), nil
}

func (median *Median) Close() error {
	return nil
}

/*
MedianOf returns the median of values without weights.
*/
func MedianOf(values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	middle := len(sorted) / 2

	if len(sorted)%2 == 1 {
		return sorted[middle], true
	}

	return (sorted[middle-1] + sorted[middle]) / 2, true
}
