package adaptive

import "github.com/theapemachine/datura"

func mergeStageOutput(state *datura.Artifact, value float64, ready bool) {
	state.MergeOutput("value", value)
	state.MergeOutput("ready", ready)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
}
