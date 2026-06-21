package geometry

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
GeometricMean combines two configured output fields with a geometric mean.
*/
type GeometricMean struct {
	config *datura.Artifact
	bytes  []byte
}

/*
NewGeometricMean returns a geometric-mean stage configured on the artifact.
*/
func NewGeometricMean(config *datura.Artifact) *GeometricMean {
	config.Inspect("geometry", "geometric-mean", "NewGeometricMean()")

	return &GeometricMean{
		config: config,
	}
}

func (geometricMean *GeometricMean) Write(p []byte) (int, error) {
	geometricMean.bytes = append(geometricMean.bytes[:0], p...)

	return len(p), nil
}

func (geometricMean *GeometricMean) Read(p []byte) (int, error) {
	state := datura.Acquire("geometric-mean-state", datura.APPJSON)
	state.Inspect("geometry", "geometric-mean", "Read()", "p")

	if _, err := state.Write(geometricMean.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	leftKey := datura.Peek[string](geometricMean.config, "inputs", "joint", "leftKey")
	rightKey := datura.Peek[string](geometricMean.config, "inputs", "joint", "rightKey")
	destinationKey := datura.Peek[string](geometricMean.config, "inputs", "joint", "destinationKey")

	if leftKey == "" || rightKey == "" || destinationKey == "" {
		return state.Read(p)
	}

	left := datura.Peek[float64](state, "output", leftKey)
	right := datura.Peek[float64](state, "output", rightKey)
	mean := 0.0

	if left > 0 && right > 0 {
		mean = math.Sqrt(left * right)
	}

	state.MergeOutput(destinationKey, mean)

	return state.Read(p)
}

func (geometricMean *GeometricMean) Close() error {
	return nil
}
