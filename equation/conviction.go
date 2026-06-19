package equation

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Conviction classifies risk-on breadth versus idiosyncratic leadership.

Payload layout: breadth, change, surgeThreshold, leader (0/1), move.
*/
type Conviction struct {
	artifact *datura.Artifact
}

/*
NewConviction returns a market-breadth conviction stage.
*/
func NewConviction() *Conviction {
	return &Conviction{
		artifact: datura.Acquire("conviction", datura.APPJSON).RetainStageAttributes(),
	}
}

func (conviction *Conviction) StageArtifact() *datura.Artifact {
	return conviction.artifact
}

func (conviction *Conviction) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](conviction.artifact, "output") == nil

	conviction.artifact.Clear("sample")

	n, err := conviction.artifact.Write(p)

	if bootstrap {
		conviction.artifact.Clear("output")
	}

	return n, err
}

func (conviction *Conviction) Read(p []byte) (int, error) {
	batch := FloatBatch(conviction.artifact)

	if len(batch) < 5 {
		conviction.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return conviction.artifact.Read(p)
	}

	breadth := finiteScore(batch[0])
	change := finiteScore(batch[1])
	surgeThreshold := finiteScore(batch[2])

	if surgeThreshold <= 0 {
		surgeThreshold = 0.5
	}

	if surgeThreshold > 1 {
		surgeThreshold = 1
	}

	leader := batch[3] > 0
	surgeScore := breadth
	divergentScore := math.Abs(change)
	slumpScore := math.Max(0, surgeThreshold-breadth)
	category := classifyConviction(breadth, change, surgeThreshold, leader)
	strength := breadth

	if category == 2 {
		strength = divergentScore
	}

	if strength <= 0 && category != 3 {
		conviction.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return conviction.artifact.Read(p)
	}

	conviction.artifact.Poke(datura.Map[float64]{
		"value":          strength,
		"surgeScore":     surgeScore,
		"divergentScore": divergentScore,
		"slumpScore":     slumpScore,
		"category":       float64(category),
		"breadth":        breadth,
		"change":         change,
	}, "output")

	return conviction.artifact.Read(p)
}

func (conviction *Conviction) Close() error {
	return nil
}

func classifyConviction(
	breadth, change, surgeThreshold float64,
	leader bool,
) int {
	if breadth >= surgeThreshold {
		return 1
	}

	if leader && change != 0 {
		return 2
	}

	return 3
}

func finiteScore(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}

	return value
}
