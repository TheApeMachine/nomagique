package statistic

import (
	"maps"
	"math"
	"strconv"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Panel registers keyed samples for cross-section pipelines.
*/
type Panel struct {
	artifact *datura.Artifact
	peers    map[string]float64
}

/*
NewPanel returns a keyed sample registry wired from config attributes on the artifact.
*/
func NewPanel(artifact *datura.Artifact) *Panel {
	return &Panel{
		artifact: artifact,
		peers:    map[string]float64{},
	}
}

func (panel *Panel) Read(payload []byte) (int, error) {
	state := datura.Acquire("panel-state", datura.APPJSON)

	if _, err := state.Unpack(panel.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"panel: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"panel: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"panel: inputs required",
			nil,
		))
	}

	memberField := datura.Peek[string](panel.artifact, "memberKey")

	if memberField == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"panel: memberKey required",
			nil,
		))
	}

	sampleField := datura.Peek[string](panel.artifact, "sampleKey")

	if sampleField == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"panel: sampleKey required",
			nil,
		))
	}

	var member float64
	var sample float64
	memberFound := false
	sampleFound := false

	for index, input := range inputs {
		var value float64

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"panel: feature index out of range",
					nil,
				))
			}

			value = features[index]
		}

		if rootKey != "features" {
			value = datura.Peek[float64](state, rootKey, input)
		}

		if input == memberField {
			member = value
			memberFound = true
		}

		if input == sampleField {
			sample = value
			sampleFound = true
		}
	}

	if !memberFound || !sampleFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"panel: member and sample keys required on wire",
			nil,
		))
	}

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"panel: sample is non-finite",
			nil,
		))
	}

	key := strconv.FormatFloat(member, 'g', -1, 64)
	panel.peers[key] = sample
	peers := maps.Clone(panel.peers)

	state.Merge("peers", peers)
	state.Merge("wire", map[string]any{
		memberField: member,
		sampleField: sample,
	})
	state.Poke("wire", "root")
	state.Poke([]string{memberField, sampleField}, "inputs")
	state.MergeOutput("value", sample)

	return state.PackInto(payload)
}

func (panel *Panel) Write(payload []byte) (int, error) {
	panel.artifact.WithPayload(payload)
	return len(payload), nil
}

func (panel *Panel) Close() error {
	return nil
}
