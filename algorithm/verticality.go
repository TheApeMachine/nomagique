package algorithm

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/learning"
	"github.com/theapemachine/nomagique/probability"
)

/*
VerticalityOutcome holds pre-pump verticality scores.
*/
type VerticalityOutcome struct {
	RVolScore        float64
	PrecursorScore   float64
	CompressionScore float64
	Move             float64
	Strength         float64
	Category         int
	Eligible         bool
}

/*
Verticality classifies volume lift, price precursor, and spread compression.

Payload layout: rvol, precursor, compression, move.
*/
type Verticality struct {
	artifact *datura.Artifact
	weights  learning.ClassifierWeights
	outcome  VerticalityOutcome
}

/*
NewVerticality returns a verticality stage with balanced classifier weights.
*/
func NewVerticality() (*Verticality, error) {
	weights, err := learning.NewClassifierWeights(1.0, learning.ClassifierFeatureScales{
		RVol:        1,
		Precursor:   1,
		Compression: 1,
	})

	if err != nil {
		return nil, err
	}

	return &Verticality{
		artifact: datura.Acquire("verticality", datura.Artifact_Type_json),
		weights:  weights,
	}, nil
}

func (verticality *Verticality) Write(p []byte) (int, error) {
	return verticality.artifact.Write(p)
}

func (verticality *Verticality) Read(p []byte) (int, error) {
	rehydrateArtifact(&verticality.artifact, "verticality", datura.Artifact_Type_json)

	payload, err := verticality.artifact.Payload()

	if err == nil {
		verticality.outcome = verticality.evaluate(payloadSamples(payload))
		verticality.publishReadings()
	}

	return verticality.artifact.Read(p)
}

func (verticality *Verticality) Close() error {
	return nil
}

/*
Outcome returns scores from the last Read.
*/
func (verticality *Verticality) Outcome() VerticalityOutcome {
	return verticality.outcome
}

func (verticality *Verticality) evaluate(batch []float64) VerticalityOutcome {
	if len(batch) < 4 {
		return VerticalityOutcome{}
	}

	rvol := boundedFeatureScore(batch[0], 1)
	precursor := boundedFeatureScore(batch[1], 0)
	compression := boundedFeatureScore(batch[2], 1)
	move := batch[3]

	scores := verticality.weights.Scores(rvol, precursor, compression)
	probabilities, probErr := probability.SoftmaxScoresNormalized(scores)

	if probErr != nil {
		return VerticalityOutcome{}
	}

	category := argmaxIndex(probabilities) + 1
	strength := verticality.weights.Strength(rvol, precursor)

	if strength <= 0 {
		return VerticalityOutcome{}
	}

	return VerticalityOutcome{
		RVolScore:        rvol,
		PrecursorScore:   precursor,
		CompressionScore: compression,
		Move:             move,
		Strength:         strength,
		Category:         category,
		Eligible:         true,
	}
}

func (verticality *Verticality) publishReadings() {
	pokeFloat(verticality.artifact, "verticality.rvol", verticality.outcome.RVolScore)
	pokeFloat(verticality.artifact, "verticality.precursor", verticality.outcome.PrecursorScore)
	pokeFloat(verticality.artifact, "verticality.compression", verticality.outcome.CompressionScore)
	pokeFloat(verticality.artifact, "verticality.strength", verticality.outcome.Strength)
	pokeFloat(verticality.artifact, "verticality.category", float64(verticality.outcome.Category))
}

func boundedFeatureScore(value, floor float64) float64 {
	if value <= floor {
		return 0
	}

	return value / (1 + value)
}

func argmaxIndex(values []float64) int {
	bestIndex := 0
	bestValue := values[0]

	for index, value := range values[1:] {
		if value > bestValue {
			bestValue = value
			bestIndex = index + 1
		}
	}

	return bestIndex
}

func (verticality *Verticality) IgnitionReading() *VerticalityReading {
	return newVerticalityReading(verticality, 0)
}

func (verticality *Verticality) CompressionReading() *VerticalityReading {
	return newVerticalityReading(verticality, 1)
}

func (verticality *Verticality) TrendReading() *VerticalityReading {
	return newVerticalityReading(verticality, 2)
}

func (verticality *Verticality) ExhaustionReading() *VerticalityReading {
	return newVerticalityReading(verticality, 3)
}

type VerticalityReading struct {
	artifact    *datura.Artifact
	verticality *Verticality
	index       int
}

func newVerticalityReading(verticality *Verticality, index int) *VerticalityReading {
	return &VerticalityReading{
		artifact:    datura.Acquire("verticality-reading", datura.Artifact_Type_json),
		verticality: verticality,
		index:       index,
	}
}

func (reading *VerticalityReading) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (reading *VerticalityReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.verticality != nil {
		scores := reading.verticality.weights.Scores(
			reading.verticality.outcome.RVolScore,
			reading.verticality.outcome.PrecursorScore,
			reading.verticality.outcome.CompressionScore,
		)

		if reading.index >= 0 && reading.index < len(scores) {
			value = scores[reading.index]
		}
	}

	_ = reading.artifact.SetPayload(encodePayload(value))

	return reading.artifact.Read(p)
}

func (reading *VerticalityReading) Close() error {
	return nil
}
