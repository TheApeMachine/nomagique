package probability_test

import (
	"github.com/bytedance/sonic"
	"github.com/theapemachine/datura"
)

func artifactWithScores(scores map[string]float64) *datura.Artifact {
	payload, err := sonic.Marshal(datura.Map[any]{"output": scores})

	if err != nil {
		panic(err)
	}

	inputs := make([]string, 0, len(scores))

	for key := range scores {
		inputs = append(inputs, key)
	}

	artifact := datura.Acquire("test", datura.APPJSON).WithPayload(payload)
	artifact.Poke("output", "root")
	artifact.Poke(inputs, "inputs")

	return artifact
}
