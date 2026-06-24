package statistic

import "github.com/theapemachine/datura"

func ScalarWire(artifact *datura.Artifact, input string, sample float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{input}, "inputs")
	artifact.Merge("features", []float64{sample})

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
