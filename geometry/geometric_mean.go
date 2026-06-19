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
	staged *datura.Artifact
}

/*
NewGeometricMean returns a geometric-mean stage configured on the artifact.
*/
func NewGeometricMean(config *datura.Artifact) *GeometricMean {
	return &GeometricMean{
		config: config,
		staged: datura.Acquire("geometric-mean", datura.APPJSON),
	}
}

func (geometricMean *GeometricMean) Write(payload []byte) (int, error) {
	return geometricMean.staged.Write(payload)
}

func (geometricMean *GeometricMean) Read(payload []byte) (int, error) {
	leftKey := datura.Peek[string](geometricMean.config, "inputs", "joint", "leftKey")
	rightKey := datura.Peek[string](geometricMean.config, "inputs", "joint", "rightKey")
	destinationKey := datura.Peek[string](geometricMean.config, "inputs", "joint", "destinationKey")

	if leftKey == "" || rightKey == "" || destinationKey == "" {
		return geometricMean.staged.Read(payload)
	}

	left := datura.Peek[float64](geometricMean.staged, "output", leftKey)
	right := datura.Peek[float64](geometricMean.staged, "output", rightKey)
	mean := 0.0

	if left > 0 && right > 0 {
		mean = math.Sqrt(left * right)
	}

	geometricMean.staged.Poke(mean, "output", destinationKey)

	return geometricMean.staged.Read(payload)
}

func (geometricMean *GeometricMean) Close() error {
	return nil
}
