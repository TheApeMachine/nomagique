package learning

import "github.com/theapemachine/datura"

func pairConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).
		Poke("sample", "sampleKey").
		Poke("paired", "pairedKey")
}

func pairWire(artifact *datura.Artifact, predicted float64, actual float64) *datura.Artifact {
	artifact.Poke("wire", "root")
	artifact.Poke([]string{"sample", "paired"}, "inputs")
	artifact.Merge("wire", map[string]any{
		"sample": predicted,
		"paired": actual,
	})

	return artifact
}

func scalarWire(artifact *datura.Artifact, input string, sample float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{input}, "inputs")
	artifact.Merge("features", []float64{sample})

	return artifact
}
