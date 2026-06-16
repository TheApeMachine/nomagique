package algorithm

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
ConvictionOutcome holds breadth-based market conviction scores.
*/
type ConvictionOutcome struct {
	Breadth        float64
	Change         float64
	Move           float64
	SurgeThreshold float64
	Leader         bool
	SurgeScore     float64
	DivergentScore float64
	SlumpScore     float64
	Strength       float64
	Category       int
	Eligible       bool
}

/*
Conviction classifies risk-on breadth versus idiosyncratic leadership.

Payload layout: breadth, change, surgeThreshold, leader (0/1), move.
*/
type Conviction struct {
	artifact *datura.Artifact
	outcome  ConvictionOutcome
}

/*
NewConviction returns a market-breadth conviction stage.
*/
func NewConviction() *Conviction {
	return &Conviction{
		artifact: datura.Acquire("conviction", datura.Artifact_Type_json),
	}
}

func (conviction *Conviction) Write(p []byte) (int, error) {
	return conviction.artifact.Write(p)
}

func (conviction *Conviction) Read(p []byte) (int, error) {
	rehydrateArtifact(&conviction.artifact, "conviction", datura.Artifact_Type_json)

	payload, err := conviction.artifact.Payload()

	if err == nil {
		conviction.outcome = conviction.evaluate(payloadSamples(payload))
		conviction.publishReadings()
	}

	return conviction.artifact.Read(p)
}

func (conviction *Conviction) Close() error {
	return nil
}

/*
Outcome returns scores from the last Read.
*/
func (conviction *Conviction) Outcome() ConvictionOutcome {
	return conviction.outcome
}

func (conviction *Conviction) evaluate(batch []float64) ConvictionOutcome {
	if len(batch) < 5 {
		return ConvictionOutcome{}
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
	move := batch[4]

	surgeScore := breadth
	divergentScore := math.Abs(change)
	slumpScore := math.Max(0, surgeThreshold-breadth)

	category := classifyConviction(breadth, change, surgeThreshold, leader)

	strength := breadth

	if category == 2 {
		strength = divergentScore
	}

	if strength <= 0 && category != 3 {
		return ConvictionOutcome{}
	}

	return ConvictionOutcome{
		Breadth:        breadth,
		Change:         change,
		Move:           move,
		SurgeThreshold: surgeThreshold,
		Leader:         leader,
		SurgeScore:     surgeScore,
		DivergentScore: divergentScore,
		SlumpScore:     slumpScore,
		Strength:       strength,
		Category:       category,
		Eligible:       category > 0,
	}
}

func (conviction *Conviction) publishReadings() {
	pokeFloat(conviction.artifact, "conviction.breadth", conviction.outcome.Breadth)
	pokeFloat(conviction.artifact, "conviction.change", conviction.outcome.Change)
	pokeFloat(conviction.artifact, "conviction.strength", conviction.outcome.Strength)
	pokeFloat(conviction.artifact, "conviction.category", float64(conviction.outcome.Category))
	pokeFloat(conviction.artifact, "conviction.surgeThreshold", conviction.outcome.SurgeThreshold)
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

func (conviction *Conviction) SurgeReading() *ConvictionReading {
	return newConvictionReading(conviction, func(outcome ConvictionOutcome) float64 {
		return outcome.SurgeScore
	})
}

func (conviction *Conviction) DivergentReading() *ConvictionReading {
	return newConvictionReading(conviction, func(outcome ConvictionOutcome) float64 {
		return outcome.DivergentScore
	})
}

func (conviction *Conviction) SlumpReading() *ConvictionReading {
	return newConvictionReading(conviction, func(outcome ConvictionOutcome) float64 {
		return outcome.SlumpScore
	})
}

type ConvictionReading struct {
	artifact   *datura.Artifact
	conviction *Conviction
	project    func(ConvictionOutcome) float64
}

func newConvictionReading(
	conviction *Conviction,
	project func(ConvictionOutcome) float64,
) *ConvictionReading {
	return &ConvictionReading{
		artifact:   datura.Acquire("conviction-reading", datura.Artifact_Type_json),
		conviction: conviction,
		project:    project,
	}
}

func (reading *ConvictionReading) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (reading *ConvictionReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.conviction != nil && reading.project != nil {
		value = reading.project(reading.conviction.outcome)
	}

	_ = reading.artifact.SetPayload(encodePayload(value))

	return reading.artifact.Read(p)
}

func (reading *ConvictionReading) Close() error {
	return nil
}
