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
	if artifact == nil {
		artifact = datura.Acquire("conviction", datura.APPJSON)
	}

	if len(datura.Peek[[]string](artifact, "inputs")) == 0 {
		artifact.Poke(ConvictionInputKeys, "inputs")
	}

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
