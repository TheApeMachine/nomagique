package equation

import (
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

const cohortPayloadHeader = 5

/*
Cohort classifies how one symbol's returns align with the peer median.

Payload layout: window, symbolReturnCount, marketReturnCount,
peerCorrelationCount, peerEnergyCount, then each series oldest→newest.
*/
type Cohort struct {
	artifact *datura.Artifact
}

/*
NewCohort returns a cross-section correlation stage.
*/
func NewCohort() *Cohort {
	return &Cohort{
		artifact: datura.Acquire("cohort", datura.APPJSON).RetainStageAttributes(),
	}
}

func (cohort *Cohort) StageArtifact() *datura.Artifact {
	return cohort.artifact
}

func (cohort *Cohort) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](cohort.artifact, "output") == nil

	cohort.artifact.Clear("sample")

	n, err := cohort.artifact.Write(p)

	if bootstrap {
		cohort.artifact.Clear("output")
	}

	return n, err
}

func (cohort *Cohort) Read(p []byte) (int, error) {
	batch := FloatBatch(cohort.artifact)
	outcome := evaluateCohort(batch)

	if !outcome.eligible || outcome.strength <= 0 {
		cohort.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return cohort.artifact.Read(p)
	}

	cohort.artifact.Poke(datura.Map[float64]{
		"value":       outcome.strength,
		"herdScore":   outcome.herdScore,
		"alphaScore":  outcome.alphaScore,
		"noiseScore":  outcome.noiseScore,
		"stressScore": outcome.stressScore,
		"category":    float64(outcome.category),
		"correlation": outcome.correlation,
		"energy":      outcome.energy,
	}, "output")

	return cohort.artifact.Read(p)
}

func (cohort *Cohort) Close() error {
	return nil
}

type cohortOutcome struct {
	correlation float64
	energy      float64
	category    int
	strength    float64
	eligible    bool
	herdScore   float64
	alphaScore  float64
	noiseScore  float64
	stressScore float64
}

func evaluateCohort(batch []float64) cohortOutcome {
	if len(batch) < cohortPayloadHeader {
		return cohortOutcome{}
	}

	window := int(batch[0])
	counts := batch[1:cohortPayloadHeader]
	offset := cohortPayloadHeader
	series := make([][]float64, len(counts))

	for index, count := range counts {
		segmentCount := int(count)

		if segmentCount < 0 || offset+segmentCount > len(batch) {
			return cohortOutcome{}
		}

		series[index] = batch[offset : offset+segmentCount]
		offset += segmentCount
	}

	symbolReturns := series[0]
	marketReturns := series[1]
	peerCorrelations := series[2]
	peerEnergies := series[3]

	if window <= 0 || len(symbolReturns) < window || len(marketReturns) < window {
		return cohortOutcome{}
	}

	if len(peerCorrelations) < 2 || len(peerEnergies) < 2 {
		return cohortOutcome{}
	}

	correlation := stat.Correlation(symbolReturns, marketReturns, nil)
	energy := cohortMedianAbsoluteValues(symbolReturns)
	upperEnergy := cohortQuantileSorted(cohortCopySorted(peerEnergies), 0.75)

	category := classifyCohort(
		correlation,
		energy,
		peerCorrelations,
		peerEnergies,
		upperEnergy,
	)

	if category == 0 {
		return cohortOutcome{}
	}

	strength := math.Abs(correlation)

	if category == 2 {
		strength = energy * (1 - math.Abs(correlation))
	}

	if strength <= 0 {
		return cohortOutcome{}
	}

	scores := cohortClassifierScores(category, correlation, energy, upperEnergy)

	return cohortOutcome{
		correlation: correlation,
		energy:      energy,
		category:    category,
		strength:    strength,
		eligible:    true,
		herdScore:   scores[0],
		alphaScore:  scores[1],
		noiseScore:  scores[2],
		stressScore: scores[3],
	}
}

func cohortClassifierScores(
	category int,
	correlation, energy, upperEnergy float64,
) [4]float64 {
	scores := [4]float64{}

	if category == 1 {
		scores[0] = correlation * energy
	}

	if category == 2 {
		scores[1] = energy * (1 - math.Abs(correlation))
	}

	if category == 3 {
		normalizedEnergy := energy

		if upperEnergy > 0 {
			normalizedEnergy = energy / upperEnergy
		}

		if normalizedEnergy > 1 {
			normalizedEnergy = 1
		}

		scores[2] = 1 - normalizedEnergy
	}

	if category == 4 {
		scores[3] = math.Abs(correlation) * energy
	}

	return scores
}

func classifyCohort(
	correlation, energy float64,
	peerCorrelations, peerEnergies []float64,
	upperEnergy float64,
) int {
	lowerCorrelation := cohortQuantileSorted(cohortCopySorted(peerCorrelations), 0.25)
	lowerEnergy := cohortQuantileSorted(cohortCopySorted(peerEnergies), 0.25)
	medianEnergy := cohortQuantileSorted(cohortCopySorted(peerEnergies), 0.5)

	energySpread := upperEnergy - lowerEnergy
	lowEnergy := energySpread > 0 && energy <= lowerEnergy

	if lowEnergy {
		return 3
	}

	upperCorrelation := cohortQuantileSorted(cohortCopySorted(peerCorrelations), 0.75)
	correlationSpread := upperCorrelation - lowerCorrelation
	highPositiveCorrelation := correlation >= upperCorrelation

	if correlationSpread <= 0 {
		highPositiveCorrelation = correlation > 0
	}

	lowMagnitudeCorrelation := peerLowMagnitudeCorrelation(
		correlation,
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
	correlationSpread float64,
	peerCorrelations []float64,
) bool {
	if len(peerCorrelations) < 3 {
		return false
	}

	magnitudes := make([]float64, len(peerCorrelations))

	for index, value := range peerCorrelations {
		magnitudes[index] = math.Abs(value)
	}

	sort.Float64s(magnitudes)

	if correlationSpread > 0 {
		lowerMagnitude := cohortQuantileSorted(magnitudes, 0.25)

		return math.Abs(correlation) <= lowerMagnitude
	}

	peerMagnitude := cohortQuantileSorted(magnitudes, 0.5)

	if peerMagnitude <= 0 {
		return false
	}

	return math.Abs(correlation) < peerMagnitude
}

func cohortCopySorted(values []float64) []float64 {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	return sorted
}

func cohortQuantileSorted(sorted []float64, percentile float64) float64 {
	if len(sorted) == 0 {
		return 0
	}

	return stat.Quantile(percentile, stat.LinInterp, sorted, nil)
}

func cohortMedianAbsoluteValues(values []float64) float64 {
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
