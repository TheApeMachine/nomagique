package equation

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
Conviction classifies risk-on breadth versus idiosyncratic leadership.
The constructor artifact holds schema inputs; Write buffers inbound wire on its payload.
*/
type Conviction struct {
	artifact *datura.Artifact
}

/*
NewConviction returns a market-breadth conviction stage wired from config attributes.
*/
func NewConviction(artifact *datura.Artifact) io.ReadWriteCloser {
	return &Conviction{
		artifact: artifact,
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

	inputKeys := EnsureFeatureSchema(state, conviction.artifact, ConvictionInputKeys)

	fields, err := FeatureFields(state, inputKeys)

	if err != nil || len(fields) < len(ConvictionInputKeys) {
		return rejectStage(state, "equation: invalid stage input")
	}

	breadth := fields[0]
	change := fields[1]
	surgeThreshold := fields[2]

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

	leader := fields[3] > 0
	category := classifyConviction(breadth, change, surgeThreshold, leader)

	surgeScore := 0.0
	divergentScore := 0.0
	slumpScore := 0.0

	switch category {
	case 1:
		surgeScore = breadth
	case 2:
		divergentScore = math.Abs(change)
	case 3:
		slumpScore = math.Max(math.Max(0, surgeThreshold-breadth), math.Abs(change))
	}

	strength := breadth

	if category == 2 {
		strength = divergentScore
	}

	if category == 3 {
		strength = slumpScore
	}

	if strength < 0 {
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
	if breadth >= surgeThreshold && leader {
		return 1
	}

	if leader && change != 0 {
		return 2
	}

	return 3
}
