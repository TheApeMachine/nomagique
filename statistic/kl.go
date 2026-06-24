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
	return &KLDivergence{
		artifact: artifact,
	}
}

func (kl *KLDivergence) Read(payload []byte) (int, error) {
	state := datura.Acquire("kl-state", datura.APPJSON)

	if _, err := state.Write(kl.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.Inspect("statistic", "kl", "Read()", "p")

	observedSample := datura.Peek[float64](state, "sample")
	expectedSample := datura.Peek[float64](state, "paired")

	if math.IsNaN(observedSample) || math.IsInf(observedSample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorNonFiniteObserved),
		))
	}

	if math.IsNaN(expectedSample) || math.IsInf(expectedSample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorNonFiniteExpected),
		))
	}

	observed := datura.Peek[[]float64](kl.artifact, "history")
	observed = append(observed, observedSample)
	kl.artifact.Poke(observed, "history")

	expected := datura.Peek[[]float64](kl.artifact, "pairedHistory")
	expected = append(expected, expectedSample)
	kl.artifact.Poke(expected, "pairedHistory")

	if len(observed) < 2 || len(expected) < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorRequireAtLeastTwoInputs),
		))
	}

	if len(observed) != len(expected) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorRequireEqualLength),
		))
	}

	divergence, err := kl.divergence(observed, expected)

	if err != nil {
		return 0, err
	}

	state.MergeOutput("value", divergence)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (kl *KLDivergence) Write(payload []byte) (int, error) {
	kl.artifact.WithPayload(payload)
	return len(payload), nil
}

func (kl *KLDivergence) Close() error {
	return nil
}

func (kl *KLDivergence) divergence(observed, expected []float64) (float64, error) {
	for _, value := range observed {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation, "unable to compute KL divergence",
				KLError(KLErrorNonFiniteObserved),
			))
		}
	}

	for _, value := range expected {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation, "unable to compute KL divergence",
				KLError(KLErrorNonFiniteExpected),
			))
		}
	}

	expectedSum := datura.Peek[float64](kl.artifact, "expectedSum")

	if expectedSum <= 0 || math.IsNaN(expectedSum) || math.IsInf(expectedSum, 0) {
		for index := range expected {
			expectedSum += expected[index]
		}
	}

	if expectedSum <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorNonFiniteExpected),
		))
	}

	observedSum := 0.0

	for index := range observed {
		observedSum += observed[index]
	}

	if math.IsNaN(observedSum) || math.IsInf(observedSum, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorNonFiniteObservedSum),
		))
	}

	if observedSum <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorNonFiniteObservedSum),
		))
	}

	width := len(observed)
	floor := kl.probabilityFloor(observed, expected, width)

	if floor < 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorNonFiniteResult),
		))
	}

	observedProbabilities := make([]float64, width)
	expectedProbabilities := make([]float64, width)

	for index := range width {
		observedProbability := observed[index] / observedSum

		if observedProbability < floor {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation, "unable to compute KL divergence",
				KLError(KLErrorNonFiniteObserved),
			))
		}

		observedProbabilities[index] = observedProbability

		expectedProbability := expected[index] / expectedSum

		if expectedProbability < floor {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation, "unable to compute KL divergence",
				KLError(KLErrorNonFiniteExpected),
			))
		}

		expectedProbabilities[index] = expectedProbability
	}

	divergence := stat.KullbackLeibler(observedProbabilities, expectedProbabilities)

	if math.IsNaN(divergence) || math.IsInf(divergence, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute KL divergence",
			KLError(KLErrorNonFiniteResult),
		))
	}

	return divergence, nil
}

func (kl *KLDivergence) probabilityFloor(
	observed, expected []float64, width int,
) float64 {
	floor := datura.Peek[float64](kl.artifact, "floor")

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
		return -1
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
