package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
)

/*
Entropy computes Shannon entropy of a normalized mass distribution over history.
*/
type Entropy struct {
	artifact *datura.Artifact
	floor    float64
}

/*
NewEntropy creates an entropy stage.
floor may be zero to derive a per-sample floor from each observation.
*/
func NewEntropy(floor float64) *Entropy {
	return &Entropy{
		artifact: datura.Acquire("entropy", datura.APPJSON).RetainStageAttributes(),
		floor:    floor,
	}
}

func (entropy *Entropy) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](entropy.artifact, "output") == nil

	entropy.artifact.Clear("sample")

	n, err := entropy.artifact.Write(p)

	if bootstrap {
		entropy.artifact.Clear("output")
	}

	return n, err
}

func (entropy *Entropy) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](entropy.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return entropy.artifact.Read(p)
	}

	history := datura.Peek[[]float64](entropy.artifact, "history")
	history = append(history, sample)
	entropy.artifact.Poke(history, "history")

	if len(history) == 0 {
		return entropy.artifact.Read(p)
	}

	probabilities, ok := entropy.normalizedProbabilities(history)

	if !ok {
		errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute entropy",
			EntropyError(EntropyErrorNonFiniteMass),
		))

		return entropy.artifact.Read(p)
	}

	value := stat.Entropy(probabilities)
	entropy.artifact.Poke(datura.Map[float64]{"value": value}, "output")

	return entropy.artifact.Read(p)
}

func (entropy *Entropy) Close() error {
	return nil
}

func (entropy *Entropy) normalizedProbabilities(values []float64) ([]float64, bool) {
	floor := entropy.probabilityFloor(values)
	total := 0.0
	masses := make([]float64, len(values))

	for index, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, false
		}

		mass := value

		if mass < floor {
			mass = floor
		}

		masses[index] = mass
		total += mass
	}

	if total <= 0 || math.IsNaN(total) || math.IsInf(total, 0) {
		return nil, false
	}

	probabilities := make([]float64, len(masses))

	for index := range masses {
		probabilities[index] = masses[index] / total
	}

	return probabilities, true
}

func (entropy *Entropy) probabilityFloor(values []float64) float64 {
	if entropy.floor > 0 {
		return entropy.floor
	}

	total := floats.Sum(values)
	scale := total / float64(len(values))

	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return math.SmallestNonzeroFloat64
	}

	return math.Nextafter(0, scale)
}

type EntropyErrorType string

const (
	EntropyErrorNonFiniteMass EntropyErrorType = "sample mass is non-finite"
)

type EntropyError string

func (entropyError EntropyError) Error() string {
	return string(entropyError)
}
