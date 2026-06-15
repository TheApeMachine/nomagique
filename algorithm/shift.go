package algorithm

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
Shift measures distribution drift between a reference stream and a live stream via KL divergence.
*/
type Shift struct {
	artifact   *datura.Artifact
	reference  []float64
	live       []float64
	weights    []float64
	expectedSum float64
	floor      float64
}

/*
NewShift creates a distribution-shift stage over reference and live streams.
expectedSum and floor may be zero to derive them from each observation.
*/
func NewShift(
	reference, live, weights []float64,
	expectedSum, floor float64,
) *Shift {
	return &Shift{
		artifact:    datura.Acquire("shift", datura.Artifact_Type_json),
		reference:   reference,
		live:        live,
		weights:     weights,
		expectedSum: expectedSum,
		floor:       floor,
	}
}

func (shift *Shift) Write(p []byte) (int, error) {
	return shift.artifact.Write(p)
}

func (shift *Shift) Read(p []byte) (int, error) {
	rehydrateArtifact(&shift.artifact, "shift", datura.Artifact_Type_json)

	reference := shift.reference
	live := shift.live

	if len(reference) == 0 || len(live) == 0 {
		return shift.artifact.Read(p)
	}

	divergence, ok := klDivergence(live, reference, shift.expectedSum, shift.floor)

	if ok {
		out := encodePayload(divergence)
		_ = shift.artifact.SetPayload(out)
	}

	return shift.artifact.Read(p)
}

func (shift *Shift) Close() error {
	return nil
}

/*
Reset clears derived state.
*/
func (shift *Shift) Reset() error {
	shift.weights = nil

	return nil
}

func klDivergence(
	observed, expected []float64, expectedSum, floor float64,
) (float64, bool) {
	for _, value := range observed {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
	}

	for _, value := range expected {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
	}

	if expectedSum <= 0 || math.IsNaN(expectedSum) || math.IsInf(expectedSum, 0) {
		for index := range expected {
			expectedSum += expected[index]
		}
	}

	width := max(len(observed), len(expected))
	probabilityFloor := klProbabilityFloor(observed, expected, width, floor)

	if expectedSum <= 0 {
		expectedSum = probabilityFloor
	}

	observedSum := 0.0

	for index := range observed {
		observedSum += observed[index]
	}

	if math.IsNaN(observedSum) || math.IsInf(observedSum, 0) || observedSum <= 0 {
		observedSum = probabilityFloor
	}

	observedProbabilities := make([]float64, width)
	expectedProbabilities := make([]float64, width)

	for index := range width {
		observedProbability := probabilityFloor

		if index < len(observed) {
			observedProbability = observed[index] / observedSum
		}

		if observedProbability < probabilityFloor {
			observedProbability = probabilityFloor
		}

		observedProbabilities[index] = observedProbability

		expectedMass := probabilityFloor

		if index < len(expected) {
			expectedMass = expected[index]
		}

		expectedProbability := expectedMass / expectedSum

		if expectedProbability < probabilityFloor {
			expectedProbability = probabilityFloor
		}

		expectedProbabilities[index] = expectedProbability
	}

	divergence := 0.0

	for index := range width {
		observedProbability := observedProbabilities[index]
		expectedProbability := expectedProbabilities[index]

		if observedProbability <= 0 || expectedProbability <= 0 {
			continue
		}

		divergence += observedProbability * math.Log(observedProbability/expectedProbability)
	}

	if math.IsNaN(divergence) || math.IsInf(divergence, 0) {
		return 0, false
	}

	return divergence, true
}

func klProbabilityFloor(
	observed, expected []float64, width int, floor float64,
) float64 {
	if floor > 0 {
		return floor
	}

	observedSum := 0.0

	for index := range observed {
		observedSum += observed[index]
	}

	expectedSum := 0.0

	for index := range expected {
		expectedSum += expected[index]
	}

	scale := math.Max(observedSum, expectedSum) / float64(width)

	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return math.SmallestNonzeroFloat64
	}

	return math.Nextafter(0, scale)
}
