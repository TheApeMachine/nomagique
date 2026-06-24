package geometry

import "github.com/theapemachine/datura"

func couplingConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).
		Poke("sample", "sampleKey").
		Poke("paired", "pairedKey")
}

func couplingWire(artifact *datura.Artifact, left float64, right float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{"sample", "paired"}, "inputs")
	artifact.Merge("features", []float64{left, right})

	return artifact
}

func velocityConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).Poke("sample", "input")
}

func velocityWire(artifact *datura.Artifact, sample float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{"sample"}, "inputs")
	artifact.Merge("features", []float64{sample})

	return artifact
}
