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

	return klDivergence(observed, expected)
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
leading none-state mass, then normalizes. When leadingMass is zero, alpha
supplies the none-state prior.
*/
func (matrix *TransitionMatrix) PadObserved(
	distribution []float64, leadingMass float64,
) ([]float64, error) {
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
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"transition: padded distribution sum is non-positive",
			nil,
		))
	}

	for index := range padded {
		padded[index] /= sum
	}

	return padded, nil
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

func klDivergence(observed, expected []float64) (float64, error) {
	if len(observed) == 0 || len(observed) != len(expected) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute KL divergence",
			KLError(KLErrorNonFiniteExpected),
		))
	}

	observedSum := 0.0

	for _, value := range observed {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"unable to compute KL divergence",
				KLError(KLErrorNonFiniteObserved),
			))
		}

		observedSum += value
	}

	if observedSum <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute KL divergence",
			KLError(KLErrorNonFiniteObservedSum),
		))
	}

	expectedSum := 0.0

	for _, value := range expected {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"unable to compute KL divergence",
				KLError(KLErrorNonFiniteExpected),
			))
		}

		expectedSum += value
	}

	if expectedSum <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"unable to compute KL divergence",
			KLError(KLErrorNonFiniteExpected),
		))
	}

	observedProbabilities := make([]float64, len(observed))
	expectedProbabilities := make([]float64, len(expected))

	for index := range observed {
		observedProbabilities[index] = observed[index] / observedSum
		expectedProbabilities[index] = expected[index] / expectedSum
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
