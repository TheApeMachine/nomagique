package statistic

import "github.com/theapemachine/datura"

/*
FeatureSnapshot captures extracted ticker columns that must survive downstream
stages once the payload root moves to output.
*/
type FeatureSnapshot struct {
	features []float64
	inputs   []string
}

/*
SnapshotFeatures records the extracted column vector from pipeline state.
*/
func SnapshotFeatures(state *datura.Artifact) FeatureSnapshot {
	return FeatureSnapshot{
		features: datura.Peek[[]float64](state, "features"),
		inputs:   datura.Peek[[]string](state, "inputs"),
	}
}

/*
Restore writes the captured feature columns back onto state.
*/
func (snapshot FeatureSnapshot) Restore(state *datura.Artifact) {
	if len(snapshot.features) > 0 {
		state.Merge("features", snapshot.features)
	}

	if len(snapshot.inputs) > 0 {
		state.Merge("inputs", snapshot.inputs)
	}
}

/*
FeatureColumn reads one extracted scalar by its schema key.
*/
func FeatureColumn(state *datura.Artifact, sourceKey string) float64 {
	snapshot := SnapshotFeatures(state)

	for index, key := range snapshot.inputs {
		if key != sourceKey || index >= len(snapshot.features) {
			continue
		}

		return snapshot.features[index]
	}

	return 0
}
