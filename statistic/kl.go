package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
KLDivergence computes sum(q_i * log(q_i / p_i)) for aligned distributions.
*/
type KLDivergence struct {
	artifact    *datura.Artifact
	expectedSum float64
	floor       float64
}

/*
NewKLDivergence creates a KL divergence stage.
expectedSum and floor may be zero to derive them from each observation.
*/
func NewKLDivergence(expectedSum, floor float64) *KLDivergence {
	return &KLDivergence{
		artifact:    datura.Acquire("kl", datura.APPJSON).RetainStageAttributes(),
		expectedSum: expectedSum,
		floor:       floor,
	}
}

func (kl *KLDivergence) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](kl.artifact, "output") == nil

	kl.artifact.Clear("sample")
	kl.artifact.Clear("paired")

	n, err := kl.artifact.Write(p)

	if bootstrap {
		kl.artifact.Clear("output")
	}

	return n, err
}

func (kl *KLDivergence) Read(p []byte) (int, error) {
	observedSample := datura.Peek[float64](kl.artifact, "sample")
	expectedSample := datura.Peek[float64](kl.artifact, "paired")

	if math.IsNaN(observedSample) || math.IsInf(observedSample, 0) {
		return kl.artifact.Read(p)
	}

	if math.IsNaN(expectedSample) || math.IsInf(expectedSample, 0) {
		return kl.artifact.Read(p)
	}

	observed := datura.Peek[[]float64](kl.artifact, "history")
	observed = append(observed, observedSample)
	kl.artifact.Poke(observed, "history")

	expected := datura.Peek[[]float64](kl.artifact, "pairedHistory")
	expected = append(expected, expectedSample)
	kl.artifact.Poke(expected, "pairedHistory")

	if len(observed) < 2 || len(expected) < 2 {
		errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorRequireAtLeastTwoInputs),
		))

		kl.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return kl.artifact.Read(p)
	}

	if len(observed) != len(expected) {
		errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorRequireEqualLength),
		))

		kl.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return kl.artifact.Read(p)
	}

	divergence, ok := kl.divergence(observed, expected)

	if !ok {
		kl.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return kl.artifact.Read(p)
	}

	kl.artifact.Poke(datura.Map[float64]{"value": divergence}, "output")

	return kl.artifact.Read(p)
}

func (kl *KLDivergence) Close() error {
	return nil
}

func (kl *KLDivergence) divergence(observed, expected []float64) (float64, bool) {
	for _, value := range observed {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			errnie.Error(errnie.Err(
				errnie.Validation, "unable to compute KL divergence",
				KLError(KLErrorNonFiniteObserved),
			))

			return 0, false
		}
	}

	for _, value := range expected {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			errnie.Error(errnie.Err(
				errnie.Validation, "unable to compute KL divergence",
				KLError(KLErrorNonFiniteExpected),
			))

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
		errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorNonFiniteObservedSum),
		))

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
		errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorNonFiniteResult),
		))

		return 0, false
	}

	return divergence, true
}

func (kl *KLDivergence) probabilityFloor(
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
