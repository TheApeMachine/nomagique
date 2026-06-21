package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
KLDivergence computes sum(q_i * log(q_i / p_i)) for aligned distributions.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type KLDivergence struct {
	artifact *datura.Artifact
}

/*
NewKLDivergence returns a KL divergence stage wired from config attributes on the artifact.
config.expectedSum and config.floor may be zero to derive them from each observation.
*/
func NewKLDivergence(artifact *datura.Artifact) *KLDivergence {
	artifact.Inspect("statistic", "kl", "NewKLDivergence()")

	return &KLDivergence{
		artifact: artifact,
	}
}

func (kl *KLDivergence) Write(payload []byte) (int, error) {
	kl.artifact.WithPayload(payload)
	return len(payload), nil
}

func (kl *KLDivergence) Read(payload []byte) (int, error) {
	state := datura.Acquire("kl-state", datura.APPJSON)
	state.Inspect("statistic", "kl", "Read()", "p")

	if _, err := state.Write(kl.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	observedSample := datura.Peek[float64](state, "sample")
	expectedSample := datura.Peek[float64](state, "paired")

	if math.IsNaN(observedSample) || math.IsInf(observedSample, 0) {
		return state.Read(payload)
	}

	if math.IsNaN(expectedSample) || math.IsInf(expectedSample, 0) {
		return state.Read(payload)
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

		state.MergeOutput("value", 0.0)
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	if len(observed) != len(expected) {
		errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorRequireEqualLength),
		))

		state.MergeOutput("value", 0.0)
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	divergence, ok := kl.divergence(observed, expected)

	if !ok {
		state.MergeOutput("value", 0.0)
		state.Merge("root", "output")
		state.Merge("inputs", []string{"value"})
		return state.Read(payload)
	}

	state.MergeOutput("value", divergence)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
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

	expectedSum := datura.Peek[float64](kl.artifact, "config", "expectedSum")

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
	floor := datura.Peek[float64](kl.artifact, "config", "floor")

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
