package equation

import (
	"io"
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
Cohort classifies one symbol's pairwise return correlation against peer correlations and energy.
The constructor artifact holds schema inputs; Write buffers inbound wire on its payload.
*/
type Cohort struct {
	artifact *datura.Artifact
}

/*
NewCohort returns a cross-section correlation stage wired from config attributes.
*/
func NewCohort(artifact *datura.Artifact) io.ReadWriteCloser {
	return &Cohort{
		artifact: artifact,
	}
}

func (cohort *Cohort) Write(p []byte) (int, error) {
	cohort.artifact.WithPayload(p)
	return len(p), nil
}

func (cohort *Cohort) Read(p []byte) (int, error) {
	state, err := stageState(cohort.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	inputKeys := EnsureFeatureSchema(state, cohort.artifact, CohortInputKeys)

	if !cohortSchemaReady(inputKeys) || len(Features(state)) < len(CohortInputKeys) {
		return rejectStage(state, "cohort: incomplete feature schema")
	}

	outcome := evaluateCohort(state, inputKeys)

	if !outcome.eligible || outcome.strength <= 0 {
		return rejectStage(state, "cohort: insufficient signal eligibility")
	}

	return emitOutput(state, p, datura.Map[float64]{
		"value":       outcome.strength,
		"herdScore":   outcome.herdScore,
		"alphaScore":  outcome.alphaScore,
		"noiseScore":  outcome.noiseScore,
		"stressScore": outcome.stressScore,
		"peakScore":   outcome.peakScore,
		"category":    float64(outcome.category),
		"correlation": outcome.correlation,
		"energy":      outcome.energy,
	})
}

func (cohort *Cohort) Close() error {
	return nil
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
	peakScore   float64
}

func evaluateCohort(state *datura.Artifact, inputKeys []string) cohortOutcome {
	fields, err := FeatureFields(state, inputKeys)

	if err != nil || len(fields) < len(CohortInputKeys) {
		return cohortOutcome{}
	}

	window := int(fields[0])
	barSpacingSeconds := fields[4]
	energy := fields[5]
	counts := fields[1:4]
	offset := len(inputKeys)
	features := Features(state)
	series := make([][]float64, len(counts))

	for index, count := range counts {
		segmentCount := int(count)

		if segmentCount < 0 || offset+segmentCount > len(features) {
			return cohortOutcome{}
		}

		series[index] = features[offset : offset+segmentCount]
		offset += segmentCount
	}

	if offset != len(features) {
		return cohortOutcome{}
	}

	pairCorrelations := series[0]
	peerCorrelations := series[1]
	peerEnergies := series[2]

	if window <= 0 || barSpacingSeconds <= 0 || len(pairCorrelations) == 0 {
		return cohortOutcome{}
	}

	if len(peerCorrelations) == 0 || len(peerEnergies) == 0 {
		return cohortOutcome{}
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
		return cohortOutcome{}
	}

	scores := cohortClassifierScores(category, correlation, energy, upperEnergy)
	strength := cohortMaxScore(scores)

	if strength <= 0 {
		return cohortOutcome{}
	}

	peakScore := 0.0

	if cohortPeakGate(correlation, peerCorrelations) {
		peakScore = math.Abs(correlation) * cohortEnergyShare(energy, upperEnergy)
	}

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
		peakScore:   peakScore,
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
