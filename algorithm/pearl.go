package algorithm

import (
	"io"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/statistic"
)

/*
NewPearl returns the Judea Pearl ladder-of-causation pipeline.
*/
func NewPearl(config *datura.Artifact) io.ReadWriteCloser {
	classifier := probability.NewClassifier(
		datura.Acquire("causal-classifier", datura.APPJSON).WithAttributes(datura.Map[any]{
			"inputs": []string{"alphaScore", "betaScore", "shockScore", "noiseScore"},
		}),
	)

	return nomagique.Number(
		NewPearlSample(config),
		causal.NewZip(config),
		statistic.NewPanel(datura.Acquire("panel-config", datura.APPJSON)),
		statistic.NewMedian(datura.Acquire("median-config", datura.APPJSON)),
		causal.NewContagion(config),
		equation.NewRegimeLadder(config),
		equation.NewCausalStory(nil),
		classifier,
	)
}
