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
