package statistic

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
FeatureSnapshot captures extracted ticker columns that must survive downstream
stages once the payload root moves to output.
*/
type FeatureSnapshot struct {
	features      []float64
	inputs        []string
	featureInputs []string
	root          string
}

/*
SnapshotFeatures records the extracted column vector from pipeline state.
*/
func SnapshotFeatures(state *datura.Artifact) FeatureSnapshot {
	snapshot := FeatureSnapshot{
		features:      datura.Peek[[]float64](state, "features"),
		inputs:        datura.Peek[[]string](state, "inputs"),
		featureInputs: datura.Peek[[]string](state, "featureInputs"),
		root:          datura.Peek[string](state, "root"),
	}

	if len(snapshot.featureInputs) == 0 && snapshot.root == "features" {
		snapshot.featureInputs = snapshot.inputs
	}

	return snapshot
}

/*
Restore writes the captured feature columns back onto state.
*/
func (snapshot FeatureSnapshot) Restore(state *datura.Artifact) {
	if len(snapshot.features) > 0 {
		state.Merge("features", snapshot.features)
	}

	if snapshot.root != "" {
		state.Poke(snapshot.root, "root")
	}

	if len(snapshot.inputs) > 0 {
		state.Poke(snapshot.inputs, "inputs")
	}

	if len(snapshot.featureInputs) > 0 {
		state.Poke(snapshot.featureInputs, "featureInputs")
	}
}

/*
FeatureColumn reads one extracted scalar by its schema key.
*/
func FeatureColumn(state *datura.Artifact, sourceKey string) (float64, error) {
	snapshot := SnapshotFeatures(state)

	for index, key := range snapshot.inputs {
		if key != sourceKey || index >= len(snapshot.features) {
			continue
		}

		return snapshot.features[index], nil
	}

	for index, key := range snapshot.featureInputs {
		if key != sourceKey || index >= len(snapshot.features) {
			continue
		}

		return snapshot.features[index], nil
	}

	return 0, errnie.Error(errnie.Err(
		errnie.Validation,
		"feature-column: key not found",
		nil,
	))
}
