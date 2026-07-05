package statistic

import (
	"github.com/theapemachine/errnie"
)

/*
FeatureSnapshot captures extracted ticker columns that must survive downstream
stages without coupling the statistic package to any transport envelope.
*/
type FeatureSnapshot struct {
	Features []float64
	Inputs   []string
	Root     string
}

/*
NewFeatureSnapshot records the extracted column vector and its schema.
*/
func NewFeatureSnapshot(inputs []string, features []float64) FeatureSnapshot {
	return FeatureSnapshot{
		Features: append([]float64(nil), features...),
		Inputs:   append([]string(nil), inputs...),
		Root:     "features",
	}
}

/*
Value reads one extracted scalar by its schema key.
*/
func (snapshot FeatureSnapshot) Value(sourceKey string) (float64, error) {
	for index, key := range snapshot.Inputs {
		if key != sourceKey || index >= len(snapshot.Features) {
			continue
		}

		return snapshot.Features[index], nil
	}

	return 0, errnie.Error(errnie.Err(
		errnie.Validation,
		"feature-column: key not found",
		nil,
	))
}

/*
FeatureColumn reads one extracted scalar by its schema key.
*/
func FeatureColumn(snapshot FeatureSnapshot, sourceKey string) (float64, error) {
	return snapshot.Value(sourceKey)
}
