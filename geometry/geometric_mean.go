package geometry

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
GeometricMean combines two configured output fields with a geometric mean.
The constructor artifact holds config; Write buffers inbound wire bytes.
*/
type GeometricMean struct {
	artifact *datura.Artifact
}

/*
NewGeometricMean returns a geometric-mean stage configured on the artifact.
*/
func NewGeometricMean(artifact *datura.Artifact) *GeometricMean {
	artifact.Inspect("geometry", "geometric-mean", "NewGeometricMean()")

	return &GeometricMean{
		artifact: artifact,
	}
}

func (geometricMean *GeometricMean) Write(p []byte) (int, error) {
	geometricMean.artifact.WithPayload(p)
	return len(p), nil
}

func (geometricMean *GeometricMean) Read(p []byte) (int, error) {
	state := datura.Acquire("geometric-mean-state", datura.APPJSON)
	state.Inspect("geometry", "geometric-mean", "Read()", "p")

	if _, err := state.Write(geometricMean.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	features := statistic.SnapshotFeatures(state)
	leftKey := datura.Peek[string](geometricMean.artifact, "joint", "leftKey")
	rightKey := datura.Peek[string](geometricMean.artifact, "joint", "rightKey")
	destinationKey := datura.Peek[string](geometricMean.artifact, "joint", "destinationKey")

	if leftKey == "" || rightKey == "" || destinationKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"geometric-mean: joint.leftKey, joint.rightKey, and joint.destinationKey are required",
			nil,
		))
	}

	left := datura.Peek[float64](state, "output", leftKey)
	right := datura.Peek[float64](state, "output", rightKey)

	if left <= 0 || right <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"geometric-mean: operands must be positive",
			nil,
		))
	}

	mean := math.Sqrt(left * right)

	state.MergeOutput(destinationKey, mean)
	features.Restore(state)
	state.Merge("root", "output")
	state.Merge("inputs", []string{destinationKey})

	return state.Read(p)
}

func (geometricMean *GeometricMean) Close() error {
	return nil
}
