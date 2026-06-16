package algorithm

import (
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/probability"
	"gonum.org/v1/gonum/stat"
)

const cohortPayloadHeader = 5

type CohortOutcome struct {
	Correlation float64
	Energy      float64
	Category    int
	Strength    float64
	Eligible    bool
}

/*
Cohort classifies how one symbol's returns align with the peer median.

Payload layout: window, symbolReturnCount, marketReturnCount,
peerCorrelationCount, peerEnergyCount, then each series oldest→newest.
*/
type Cohort struct {
	artifact *datura.Artifact
	outcome  CohortOutcome
}

/*
NewCohort returns a cross-section correlation stage.
*/
func NewCohort() *Cohort {
	return &Cohort{
		artifact: datura.Acquire("cohort", datura.Artifact_Type_json),
	}
}

func (cohort *Cohort) Write(p []byte) (int, error) {
	return cohort.artifact.Write(p)
}

func (cohort *Cohort) Read(p []byte) (int, error) {
	rehydrateArtifact(&cohort.artifact, "cohort", datura.Artifact_Type_json)

	payload, err := cohort.artifact.Payload()

	if err == nil {
		cohort.outcome = cohort.evaluate(payloadSamples(payload))
		cohort.publishReadings()
	}

	return cohort.artifact.Read(p)
}

func (cohort *Cohort) Close() error {
	return nil
}

/*
Outcome returns scores from the last Read.
*/
func (cohort *Cohort) Outcome() CohortOutcome {
	return cohort.outcome
}

func (cohort *Cohort) evaluate(batch []float64) CohortOutcome {
	if len(batch) < cohortPayloadHeader {
		return CohortOutcome{}
	}

	window := int(batch[0])
	counts := batch[1:cohortPayloadHeader]
	offset := cohortPayloadHeader
	series := make([][]float64, len(counts))

	for index, count := range counts {
		segmentCount := int(count)

		if segmentCount < 0 || offset+segmentCount > len(batch) {
			return CohortOutcome{}
		}

		series[index] = batch[offset : offset+segmentCount]
		offset += segmentCount
	}

	symbolReturns := series[0]
	marketReturns := series[1]
	peerCorrelations := series[2]
	peerEnergies := series[3]

	if window <= 0 || len(symbolReturns) < window || len(marketReturns) < window {
		return CohortOutcome{}
	}

	if len(peerCorrelations) < 2 || len(peerEnergies) < 2 {
		return CohortOutcome{}
	}

	correlation := stat.Correlation(symbolReturns, marketReturns, nil)
	energy := medianAbsoluteValues(symbolReturns)
	upperEnergy := quantileSorted(copySorted(peerEnergies), 0.75)

	category := classifyCohort(
		correlation,
		energy,
		peerCorrelations,
		peerEnergies,
		upperEnergy,
	)

	if category == 0 {
		return CohortOutcome{}
	}

	strength := math.Abs(correlation)

	if category == 2 {
		strength = energy * (1 - math.Abs(correlation))
	}

	if strength <= 0 {
		return CohortOutcome{}
	}

	return CohortOutcome{
		Correlation: correlation,
		Energy:      energy,
		Category:    category,
		Strength:    strength,
		Eligible:    true,
	}
}

func (cohort *Cohort) publishReadings() {
	pokeFloat(cohort.artifact, "cohort.correlation", cohort.outcome.Correlation)
	pokeFloat(cohort.artifact, "cohort.energy", cohort.outcome.Energy)
	pokeFloat(cohort.artifact, "cohort.strength", cohort.outcome.Strength)
}

func (cohort *Cohort) HerdReading() *CohortReading {
	return newCohortReading(cohort, func(outcome CohortOutcome) float64 {
		if outcome.Category != 1 {
			return 0
		}

		return outcome.Correlation * outcome.Energy
	})
}

func (cohort *Cohort) AlphaReading() *CohortReading {
	return newCohortReading(cohort, func(outcome CohortOutcome) float64 {
		if outcome.Category != 2 {
			return 0
		}

		return outcome.Energy * (1 - math.Abs(outcome.Correlation))
	})
}

func (cohort *Cohort) NoiseReading() *CohortReading {
	return newCohortReading(cohort, func(outcome CohortOutcome) float64 {
		if outcome.Category != 3 {
			return 0
		}

		return 1 - math.Min(outcome.Energy, 1)
	})
}

func (cohort *Cohort) StressReading() *CohortReading {
	return newCohortReading(cohort, func(outcome CohortOutcome) float64 {
		if outcome.Category != 4 {
			return 0
		}

		return math.Abs(outcome.Correlation) * outcome.Energy
	})
}

type CohortReading struct {
	artifact *datura.Artifact
	cohort   *Cohort
	project  func(CohortOutcome) float64
}

func newCohortReading(
	cohort *Cohort,
	project func(CohortOutcome) float64,
) *CohortReading {
	return &CohortReading{
		artifact: datura.Acquire("cohort-reading", datura.Artifact_Type_json),
		cohort:   cohort,
		project:  project,
	}
}

func (reading *CohortReading) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (reading *CohortReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.cohort != nil && reading.project != nil {
		value = reading.project(reading.cohort.outcome)
	}

	_ = reading.artifact.SetPayload(encodePayload(value))

	return reading.artifact.Read(p)
}

func (reading *CohortReading) Close() error {
	return nil
}

func classifyCohort(
	correlation, energy float64,
	peerCorrelations, peerEnergies []float64,
	upperEnergy float64,
) int {
	lowerCorrelation := quantileSorted(copySorted(peerCorrelations), 0.25)
	lowerEnergy := quantileSorted(copySorted(peerEnergies), 0.25)
	medianEnergy := quantileSorted(copySorted(peerEnergies), 0.5)

	energySpread := upperEnergy - lowerEnergy
	lowEnergy := energySpread > 0 && energy <= lowerEnergy

	if lowEnergy {
		return 3
	}

	upperCorrelation := quantileSorted(copySorted(peerCorrelations), 0.75)
	correlationSpread := upperCorrelation - lowerCorrelation
	highPositiveCorrelation := correlation >= upperCorrelation

	if correlationSpread <= 0 {
		highPositiveCorrelation = correlation > 0
	}

	lowMagnitudeCorrelation := peerLowMagnitudeCorrelation(
		correlation,
		lowerCorrelation,
		correlationSpread,
		peerCorrelations,
	)

	highEnergy := energy >= upperEnergy

	if energySpread <= 0 {
		highEnergy = energy >= medianEnergy
	}

	if correlation < 0 && highEnergy && math.Abs(correlation) >= math.Abs(lowerCorrelation) {
		return 4
	}

	if highPositiveCorrelation && highEnergy {
		return 1
	}

	if lowMagnitudeCorrelation && highEnergy {
		return 2
	}

	return 3
}

func peerLowMagnitudeCorrelation(
	correlation float64,
	lowerCorrelation float64,
	correlationSpread float64,
	peerCorrelations []float64,
) bool {
	if correlationSpread > 0 {
		return math.Abs(correlation) <= lowerCorrelation
	}

	if len(peerCorrelations) < 3 {
		return false
	}

	peerMagnitude := medianAbsoluteValues(peerCorrelations)

	if peerMagnitude <= 0 {
		return false
	}

	return math.Abs(correlation) < peerMagnitude
}

func copySorted(values []float64) []float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	return sorted
}

func quantileSorted(sorted []float64, percentile float64) float64 {
	if len(sorted) == 0 {
		return 0
	}

	return stat.Quantile(percentile, stat.LinInterp, sorted, nil)
}

func medianAbsoluteValues(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	absValues := make([]float64, len(values))

	for index, value := range values {
		absValues[index] = math.Abs(value)
	}

	sort.Float64s(absValues)

	return stat.Quantile(0.5, stat.LinInterp, absValues, nil)
}

func (cohort *Cohort) classifierScores(outcome CohortOutcome, upperEnergy float64) []float64 {
	herdScore := 0.0

	if outcome.Category == 1 {
		herdScore = outcome.Correlation * outcome.Energy
	}

	alphaScore := 0.0

	if outcome.Category == 2 {
		alphaScore = outcome.Energy * (1 - math.Abs(outcome.Correlation))
	}

	noiseScore := 0.0

	if outcome.Category == 3 {
		normalizedEnergy := outcome.Energy

		if upperEnergy > 0 {
			normalizedEnergy = outcome.Energy / upperEnergy
		}

		if normalizedEnergy > 1 {
			normalizedEnergy = 1
		}

		noiseScore = 1 - normalizedEnergy
	}

	stressScore := 0.0

	if outcome.Category == 4 {
		stressScore = math.Abs(outcome.Correlation) * outcome.Energy
	}

	return []float64{herdScore, alphaScore, noiseScore, stressScore}
}

func (cohort *Cohort) ClassifierProbabilities(outcome CohortOutcome, upperEnergy float64) ([]float64, error) {
	return probability.SoftmaxScoresNormalized(cohort.classifierScores(outcome, upperEnergy))
}
