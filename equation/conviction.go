package equation

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
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
func NewConviction() io.ReadWriteCloser {
	return &Conviction{
		artifact: datura.Acquire("conviction", datura.APPJSON),
	}
}

func (conviction *Conviction) Write(p []byte) (int, error) {
	conviction.artifact.WithPayload(p)
	return len(p), nil
}

func (conviction *Conviction) Read(p []byte) (int, error) {
	state, err := stageState(conviction.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	batch := Features(state)

	if len(batch) < 5 {
		return rejectStage(state, "equation: invalid stage input")
	}

	breadth := batch[0]
	change := batch[1]
	surgeThreshold := batch[2]

	for _, value := range []float64{breadth, change, surgeThreshold} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return rejectStage(state, "equation: invalid stage input")
		}
	}

	if surgeThreshold <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"conviction: surgeThreshold must be positive",
			nil,
		))
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
		return rejectStage(state, "equation: invalid stage input")
	}

	return emitOutput(state, p, datura.Map[float64]{
		"value":          strength,
		"surgeScore":     surgeScore,
		"divergentScore": divergentScore,
		"slumpScore":     slumpScore,
		"category":       float64(category),
		"breadth":        breadth,
		"change":         change,
	})
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
