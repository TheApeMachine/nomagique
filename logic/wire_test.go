package logic_test

import "github.com/theapemachine/datura"

func scalarWire(artifact *datura.Artifact, sample float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{"sample"}, "inputs")
	artifact.Merge("features", []float64{sample})

	return artifact
}
