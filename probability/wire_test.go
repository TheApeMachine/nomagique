package probability

import "github.com/theapemachine/datura"

func scalarWire(artifact *datura.Artifact, input string, sample float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{input}, "inputs")
	artifact.Merge("features", []float64{sample})

	return artifact
}

func cusumConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).Poke("sample", "sampleKey")
}

func rankConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).Poke("sample", "sampleKey")
}

func bernoulliConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).Poke("sample", "sampleKey")
}

func bernoulliPairConfig(name string) *datura.Artifact {
	return bernoulliConfig(name).Poke("paired", "pairedKey")
}

func pairWire(
	artifact *datura.Artifact,
	sampleKey string,
	pairedKey string,
	sample float64,
	paired float64,
) *datura.Artifact {
	artifact.Poke("wire", "root")
	artifact.Poke([]string{sampleKey, pairedKey}, "inputs")
	artifact.Merge("wire", map[string]any{
		sampleKey: sample,
		pairedKey: paired,
	})

	return artifact
}
