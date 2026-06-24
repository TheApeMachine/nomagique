package equation

import (
	"io"
	"math"
	"sort"
	"time"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
Cohort classifies how one symbol's returns align with the peer median.
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
	barSpacingSeconds := fields[5]
	counts := fields[1:5]
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

	if hyCorrelation, hyOK := cohortHayashiCorrelation(symbolReturns, marketReturns, barSpacingSeconds); hyOK {
		if math.Abs(correlation) < math.Abs(hyCorrelation) || math.IsNaN(correlation) {
			correlation = hyCorrelation
		}
	}
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
	peakScore := 0.0

	if cohortPeakGate(correlation, peerCorrelations) {
		peakScore = math.Abs(correlation) * energy
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

	if highPositiveCorrelation && highEnergy && cohortPeakGate(correlation, peerCorrelations) {
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

func cohortHayashiCorrelation(left, right []float64, barSpacingSeconds float64) (float64, bool) {
	if barSpacingSeconds <= 0 || math.IsNaN(barSpacingSeconds) || math.IsInf(barSpacingSeconds, 0) {
		return 0, false
	}

	if len(left) < 2 || len(left) != len(right) {
		return 0, false
	}

	barSpacing := time.Duration(barSpacingSeconds * float64(time.Second))
	leftSamples := make([]cohortHayashiSample, len(left))
	rightSamples := make([]cohortHayashiSample, len(right))
	start := time.Unix(0, 0)

	for index := range left {
		at := start.Add(time.Duration(index) * barSpacing)
		leftSamples[index] = cohortHayashiSample{At: at, Value: left[index]}
		rightSamples[index] = cohortHayashiSample{At: at, Value: right[index]}
	}

	return cohortHayashiYoshida(leftSamples, rightSamples, barSpacing*2)
}

type cohortHayashiSample struct {
	At    time.Time
	Value float64
}

func cohortHayashiYoshida(
	left, right []cohortHayashiSample,
	maxInterval time.Duration,
) (float64, bool) {
	if len(left) < 2 || len(right) < 2 {
		return 0, false
	}

	leftVariance := cohortHayashiVariance(left, maxInterval)
	rightVariance := cohortHayashiVariance(right, maxInterval)

	if leftVariance <= 0 || rightVariance <= 0 {
		return 0, false
	}

	covariance := 0.0
	rightStart := 0

	for leftIndex := 0; leftIndex < len(left)-1; leftIndex++ {
		leftStart := left[leftIndex].At
		leftEnd := left[leftIndex+1].At

		if !cohortHayashiInterval(left[leftIndex], left[leftIndex+1], maxInterval) {
			continue
		}

		leftReturn := math.Log(left[leftIndex+1].Value / left[leftIndex].Value)

		for rightStart < len(right)-1 {
			if !cohortHayashiInterval(right[rightStart], right[rightStart+1], maxInterval) ||
				!leftStart.Before(right[rightStart+1].At) {
				rightStart++

				continue
			}

			break
		}

		for rightIndex := rightStart; rightIndex < len(right)-1; rightIndex++ {
			rightIntervalStart := right[rightIndex].At

			if !rightIntervalStart.Before(leftEnd) {
				break
			}

			if !cohortHayashiInterval(right[rightIndex], right[rightIndex+1], maxInterval) {
				continue
			}

			covariance += leftReturn * math.Log(
				right[rightIndex+1].Value/right[rightIndex].Value,
			)
		}
	}

	denominator := math.Sqrt(leftVariance * rightVariance)

	if denominator <= 0 {
		return 0, false
	}

	correlationValue := covariance / denominator

	if correlationValue > 1 {
		return 1, true
	}

	if correlationValue < -1 {
		return -1, true
	}

	return correlationValue, true
}

func cohortHayashiVariance(samples []cohortHayashiSample, maxInterval time.Duration) float64 {
	if len(samples) < 2 {
		return 0
	}

	sum := 0.0

	for index := 1; index < len(samples); index++ {
		if !cohortHayashiInterval(samples[index-1], samples[index], maxInterval) {
			continue
		}

		ret := math.Log(samples[index].Value / samples[index-1].Value)
		sum += ret * ret
	}

	return sum
}

func cohortHayashiInterval(
	previous, current cohortHayashiSample,
	maxInterval time.Duration,
) bool {
	if previous.Value <= 0 || current.Value <= 0 || !previous.At.Before(current.At) {
		return false
	}

	if maxInterval <= 0 {
		return true
	}

	return current.At.Sub(previous.At) <= maxInterval
}
