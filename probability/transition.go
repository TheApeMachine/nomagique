package probability

import (
	"math"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
TransitionMatrix tracks category transitions with Dirichlet-smoothed counts and
scores surprisal via KL divergence against an observed distribution.
*/
type TransitionMatrix struct {
	counts         [][]float64
	lastCategory   int
	numStates      int
	smoothingAlpha float64
}

/*
NewTransitionMatrix creates a transition matrix with Dirichlet smoothing alpha.
*/
func NewTransitionMatrix(numStates int, alpha float64) *TransitionMatrix {
	counts := make([][]float64, numStates)

	for row := range counts {
		counts[row] = make([]float64, numStates)

		for column := range counts[row] {
			counts[row][column] = alpha
		}
	}

	return &TransitionMatrix{
		counts:         counts,
		lastCategory:   0,
		numStates:      numStates,
		smoothingAlpha: alpha,
	}
}

/*
Surprise scores KL divergence between observed and the current transition row.
*/
func (matrix *TransitionMatrix) Surprise(observed []float64) (float64, error) {
	row := matrix.counts[matrix.lastCategory]
	rowSum := 0.0

	for _, count := range row {
		rowSum += count
	}

	if rowSum <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"transition: row sum is non-positive",
			nil,
		))
	}

	expected := make([]float64, len(row))

	for index, count := range row {
		expected[index] = count / rowSum
	}

	return klDivergence(observed, expected, 0, 0)
}

/*
Update records a transition into stateIndex.
*/
func (matrix *TransitionMatrix) Update(stateIndex int) {
	matrix.counts[matrix.lastCategory][stateIndex] += 1.0
	matrix.lastCategory = stateIndex
}

/*
PadObserved maps an N-category distribution into a numStates vector with a
leading none-state mass, then normalizes. When leadingMass is zero, smoothingAlpha
supplies the none-state prior.
*/
func (matrix *TransitionMatrix) PadObserved(
	distribution []float64, leadingMass float64,
) []float64 {
	if leadingMass <= 0 {
		leadingMass = matrix.smoothingAlpha
	}

	padded := make([]float64, matrix.numStates)
	padded[0] = leadingMass

	for index, probability := range distribution {
		target := index + 1

		if target >= matrix.numStates {
			break
		}

		padded[target] = probability
	}

	sum := 0.0

	for _, probability := range padded {
		sum += probability
	}

	if sum <= 0 {
		return padded
	}

	for index := range padded {
		padded[index] /= sum
	}

	return padded
}

/*
Reset clears transition counts back to the smoothing prior.
*/
func (matrix *TransitionMatrix) Reset() {
	for row := range matrix.counts {
		for column := range matrix.counts[row] {
			matrix.counts[row][column] = matrix.smoothingAlpha
		}
	}

	matrix.lastCategory = 0
}

func klDivergence(
	observed, expected []float64, expectedSum, floor float64,
) (float64, error) {
	for _, value := range observed {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"unable to compute KL divergence",
				KLError(KLErrorNonFiniteObserved),
			))
		}
	}

	for _, value := range expected {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"unable to compute KL divergence",
				KLError(KLErrorNonFiniteExpected),
			))
		}
	}

	if expectedSum <= 0 || math.IsNaN(expectedSum) || math.IsInf(expectedSum, 0) {
		for index := range expected {
			expectedSum += expected[index]
		}
	}

	width := max(len(observed), len(expected))
	probabilityFloor := klProbabilityFloor(observed, expected, width, floor)

	if probabilityFloor <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute KL divergence",
			KLError(KLErrorNonFiniteExpected),
		))
	}

	if expectedSum <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute KL divergence",
			KLError(KLErrorNonFiniteExpected),
		))
	}

	observedSum := 0.0

	for index := range observed {
		observedSum += observed[index]
	}

	if math.IsNaN(observedSum) || math.IsInf(observedSum, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute KL divergence",
			KLError(KLErrorNonFiniteObservedSum),
		))
	}

	if observedSum <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute KL divergence",
			KLError(KLErrorNonFiniteObservedSum),
		))
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

	observedTotal := 0.0
	expectedTotal := 0.0

	for index := range width {
		observedTotal += observedProbabilities[index]
		expectedTotal += expectedProbabilities[index]
	}

	if observedTotal <= 0 || expectedTotal <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute KL divergence",
			KLError(KLErrorNonFiniteResult),
		))
	}

	for index := range width {
		observedProbabilities[index] /= observedTotal
		expectedProbabilities[index] /= expectedTotal
	}

	divergence := stat.KullbackLeibler(observedProbabilities, expectedProbabilities)

	if math.IsNaN(divergence) || math.IsInf(divergence, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute KL divergence",
			KLError(KLErrorNonFiniteResult),
		))
	}

	return divergence, nil
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
		return 0
	}

	return math.Nextafter(0, scale)
}

type KLErrorType string

const (
	KLErrorNonFiniteObserved    KLErrorType = "observed sample is non-finite"
	KLErrorNonFiniteExpected    KLErrorType = "expected sample is non-finite"
	KLErrorNonFiniteObservedSum KLErrorType = "observed sum is non-finite"
	KLErrorNonFiniteResult      KLErrorType = "kl divergence is non-finite"
)

type KLError string

func (klError KLError) Error() string {
	return string(klError)
}
