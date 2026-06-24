package statistic

import "github.com/theapemachine/datura"

func ScalarWire(artifact *datura.Artifact, input string, sample float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{input}, "inputs")
	artifact.Merge("features", []float64{sample})

	return artifact
}

func PairWire(
	artifact *datura.Artifact,
	sampleKey string,
	pairedKey string,
	sample float64,
	paired float64,
) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{sampleKey, pairedKey}, "inputs")
	artifact.Merge("features", []float64{sample, paired})

	return artifact
}

func PanelWire(artifact *datura.Artifact, member float64, sample float64) *datura.Artifact {
	artifact.Poke("wire", "root")
	artifact.Poke([]string{"member", "sample"}, "inputs")
	artifact.Merge("wire", map[string]any{
		"member": member,
		"sample": sample,
	})

	return artifact
}

func scalarStageConfig(name string) *datura.Artifact {
	return datura.Acquire(name, datura.APPJSON).
		Poke("sample", "input").
		Poke("value", "outputKey")
}
