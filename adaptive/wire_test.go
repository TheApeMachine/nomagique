package adaptive

import "github.com/theapemachine/datura"

func ScalarWire(artifact *datura.Artifact, input string, sample float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{input}, "inputs")
	artifact.Merge("features", []float64{sample})

	return artifact
}
