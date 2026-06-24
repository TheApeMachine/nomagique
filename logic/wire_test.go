package logic_test

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/logic"
)

func constantStage(value float64) *logic.Constant {
	return logic.NewConstant(
		datura.Acquire("constant-config", datura.APPJSON).Poke(value, "value"),
	)
}

func scalarWire(artifact *datura.Artifact, sample float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{"sample"}, "inputs")
	artifact.Merge("features", []float64{sample})

	return artifact
}

func circuitConfig() *datura.Artifact {
	return datura.Acquire("circuit-config", datura.APPJSON)
}
