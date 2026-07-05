package equation

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Cohort classifies one symbol's pairwise return correlation against peer correlations and energy.
*/
type Cohort struct{}

/*
NewCohort returns a typed cross-section correlation classifier.
*/
func NewCohort() *Cohort {
	return &Cohort{}
}

func cohortSchemaReady(inputKeys []string) bool {
	if len(inputKeys) < len(CohortInputKeys) {
		return false
	}

	for _, expected := range CohortInputKeys {
		found := false

		for _, actual := range inputKeys {
			if actual == expected {
				found = true

				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

/*
CohortOutput carries typed classifier evidence.
*/
type CohortOutput struct {
	Correlation float64
	Energy      float64
	Category    int
	Strength    float64
	Eligible    bool
	HerdScore   float64
	AlphaScore  float64
	NoiseScore  float64
	StressScore float64
	PeakScore   float64
}

/*
Measure classifies a feature frame using its semantic feature schema.
*/
func (cohort *Cohort) Measure(frame FeatureFrame) (CohortOutput, error) {
	if !cohortSchemaReady(frame.Inputs) || len(frame.Features) < len(CohortInputKeys) {
		return CohortOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"cohort: incomplete feature schema",
			nil,
		))
	}

	outcome := evaluateCohort(frame, frame.Inputs)

	if !outcome.Eligible || outcome.Strength <= 0 {
		return CohortOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"cohort: insufficient signal eligibility",
			nil,
		))
	}

	return outcome, nil
}

func evaluateCohort(frame FeatureFrame, inputKeys []string) CohortOutput {
	fields, err := FeatureFields(frame, inputKeys)

	if err != nil || len(fields) < len(CohortInputKeys) {
		return CohortOutput{}
	}

	window := int(fields[0])
	barSpacingSeconds := fields[4]
	energy := fields[5]
	counts := fields[1:4]
	offset := len(inputKeys)
	series := make([][]float64, len(counts))

	for index, count := range counts {
		segmentCount := int(count)

		if segmentCount < 0 || offset+segmentCount > len(frame.Features) {
			return CohortOutput{}
		}

		series[index] = frame.Features[offset : offset+segmentCount]
		offset += segmentCount
	}

	if offset != len(frame.Features) {
		return CohortOutput{}
	}

	pairCorrelations := series[0]
	peerCorrelations := series[1]
	peerEnergies := series[2]

	if window <= 0 || barSpacingSeconds <= 0 || len(pairCorrelations) == 0 {
		return CohortOutput{}
	}

	if len(peerCorrelations) == 0 || len(peerEnergies) == 0 {
		return CohortOutput{}
	}

	correlation := cohortQuantileSorted(cohortCopySorted(pairCorrelations), 0.5)
	upperEnergy := cohortQuantileSorted(cohortCopySorted(peerEnergies), 0.75)

	category := classifyCohort(
		correlation,
		energy,
		peerCorrelations,
		peerEnergies,
		upperEnergy,
	)

	if category == 0 {
		return CohortOutput{}
	}

	scores := cohortClassifierScores(category, correlation, energy, upperEnergy)
	strength := cohortMaxScore(scores)

	if strength <= 0 {
		return CohortOutput{}
	}

	peakScore := 0.0

	if cohortPeakGate(correlation, peerCorrelations) {
		peakScore = math.Abs(correlation) * cohortEnergyShare(energy, upperEnergy)
	}

	return CohortOutput{
		Correlation: correlation,
		Energy:      energy,
		Category:    category,
		Strength:    strength,
		Eligible:    true,
		HerdScore:   scores[0],
		AlphaScore:  scores[1],
		NoiseScore:  scores[2],
		StressScore: scores[3],
		PeakScore:   peakScore,
	}
}

func cohortPeakGate(correlation float64, peerCorrelations []float64) bool {
	if len(peerCorrelations) < 2 {
		return correlation > 0
	}

	peakQuantile := cohortQuantileSorted(cohortCopySorted(peerCorrelations), 0.9)

	if peakQuantile <= 0 {
		return correlation > 0
	}

	return correlation >= peakQuantile
}

func cohortClassifierScores(
	category int,
	correlation, energy, upperEnergy float64,
) [4]float64 {
	scores := [4]float64{}
	energyShare := cohortEnergyShare(energy, upperEnergy)

	if category == 1 {
		scores[0] = math.Max(0, correlation) * energyShare
	}

	if category == 2 {
		scores[1] = math.Max(0, 1-math.Abs(correlation)) * energyShare
	}

	if category == 3 {
		scores[2] = 1 - energyShare
	}

	if category == 4 {
		scores[3] = math.Abs(correlation) * energyShare
	}

	return scores
}

func cohortEnergyShare(energy, upperEnergy float64) float64 {
	if energy <= 0 || math.IsNaN(energy) || math.IsInf(energy, 0) {
		return 0
	}

	if upperEnergy <= 0 || math.IsNaN(upperEnergy) || math.IsInf(upperEnergy, 0) {
		return 1
	}

	return math.Min(energy/upperEnergy, 1)
}

func cohortMaxScore(scores [4]float64) float64 {
	best := 0.0

	for _, score := range scores {
		if score > best && !math.IsNaN(score) && !math.IsInf(score, 0) {
			best = score
		}
	}

	return best
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
	lowEnergy := energy <= lowerEnergy

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

	highEnergy := energy >= medianEnergy && energy > 0

	if energySpread <= 0 {
		lowEnergy = energy <= medianEnergy
	}

	if correlation < 0 && highEnergy {
		magnitudes := make([]float64, len(peerCorrelations))

		for index, value := range peerCorrelations {
			magnitudes[index] = math.Abs(value)
		}

		sort.Float64s(magnitudes)

		if math.Abs(correlation) >= cohortQuantileSorted(magnitudes, 0.75) {
			return 4
		}
	}

	if highPositiveCorrelation && highEnergy {
		return 1
	}

	if lowMagnitudeCorrelation && highEnergy {
		return 2
	}

	if lowEnergy || lowMagnitudeCorrelation {
		return 3
	}

	return 0
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
