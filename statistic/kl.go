package statistic

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
KLDivergence computes sum(q_i * log(q_i / p_i)) for aligned distributions.
Inputs split into observed and expected halves; lengths may differ inside each half.
When expectedSum or floor on the receiver are zero, they are derived from the samples.
*/
type KLDivergence[T ~float64] struct {
	weights     []float64
	expectedSum float64
	floor       float64
	output      core.Scalar[T]
}

/*
NewKLDivergence creates a KL divergence stage.
expectedSum and floor may be zero to derive them from each observation.
*/
func NewKLDivergence[T ~float64](
	weights []float64,
	expectedSum, floor float64,
) *KLDivergence[T] {
	return &KLDivergence[T]{
		weights:     weights,
		expectedSum: expectedSum,
		floor:       floor,
	}
}

/*
Observe computes KL divergence between observed and expected sample halves.
*/
func (kl *KLDivergence[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	count := len(inputs)

	if count < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorRequireAtLeastTwoInputs),
		)

		return kl.output
	}

	if count%2 != 0 {
		errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorRequireEqualLength),
		)

		return kl.output
	}

	half := count / 2
	observed := sampleBatch[T](inputs[:half]...)
	expected := sampleBatch[T](inputs[half:]...)

	divergence, ok := kl.divergence(observed, expected)

	if !ok {
		return kl.output
	}

	kl.output = core.Scalar[T](T(divergence))

	return kl.output
}

/*
Reset clears derived state.
*/
func (kl *KLDivergence[T]) Reset() error {
	kl.weights = nil
	kl.output = core.Scalar[T](0)

	return nil
}

func (kl *KLDivergence[T]) divergence(observed, expected []float64) (float64, bool) {
	for _, value := range observed {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			errnie.Err(
				errnie.Validation, "unable to compute KL divergence",
				KLError(KLErrorNonFiniteObserved),
			)

			return 0, false
		}
	}

	for _, value := range expected {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			errnie.Err(
				errnie.Validation, "unable to compute KL divergence",
				KLError(KLErrorNonFiniteExpected),
			)

			return 0, false
		}
	}

	expectedSum := kl.expectedSum

	if expectedSum <= 0 || math.IsNaN(expectedSum) || math.IsInf(expectedSum, 0) {
		for index := range expected {
			expectedSum += expected[index]
		}
	}

	width := max(len(observed), len(expected))
	floor := kl.probabilityFloor(observed, expected, width)

	if expectedSum <= 0 {
		expectedSum = floor
	}

	observedSum := 0.0

	for index := range observed {
		observedSum += observed[index]
	}

	if math.IsNaN(observedSum) || math.IsInf(observedSum, 0) {
		errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorNonFiniteObservedSum),
		)

		return 0, false
	}

	if observedSum <= 0 {
		observedSum = floor
	}

	observedProbabilities := make([]float64, width)
	expectedProbabilities := make([]float64, width)

	for index := range width {
		observedProbability := floor

		if index < len(observed) {
			observedProbability = observed[index] / observedSum
		}

		if observedProbability < floor {
			observedProbability = floor
		}

		observedProbabilities[index] = observedProbability

		expectedMass := floor

		if index < len(expected) {
			expectedMass = expected[index]
		}

		expectedProbability := expectedMass / expectedSum

		if expectedProbability < floor {
			expectedProbability = floor
		}

		expectedProbabilities[index] = expectedProbability
	}

	divergence := stat.KullbackLeibler(observedProbabilities, expectedProbabilities)

	if math.IsNaN(divergence) || math.IsInf(divergence, 0) {
		errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorNonFiniteResult),
		)

		return 0, false
	}

	return divergence, true
}

func (kl *KLDivergence[T]) probabilityFloor(
	observed, expected []float64, width int,
) float64 {
	if kl.floor > 0 {
		return kl.floor
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

type KLErrorType string

const (
	KLErrorRequireAtLeastTwoInputs KLErrorType = "require at least two inputs"
	KLErrorRequireEqualLength      KLErrorType = "require equal length"
	KLErrorNonFiniteObserved       KLErrorType = "observed sample is non-finite"
	KLErrorNonFiniteExpected       KLErrorType = "expected sample is non-finite"
	KLErrorNonFiniteObservedSum    KLErrorType = "observed sum is non-finite"
	KLErrorNonFiniteResult         KLErrorType = "kl divergence is non-finite"
)

type KLError string

func (klError KLError) Error() string {
	return string(klError)
}
