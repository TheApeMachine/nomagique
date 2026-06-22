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
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Entropy struct {
	artifact *datura.Artifact
}

/*
NewEntropy returns an entropy stage wired from config attributes on the artifact.
config.floor may be zero to derive a per-sample floor from each observation.
*/
func NewEntropy(artifact *datura.Artifact) *Entropy {
	artifact.Inspect("statistic", "entropy", "NewEntropy()")

	return &Entropy{
		artifact: artifact,
	}
}

func (entropy *Entropy) Write(payload []byte) (int, error) {
	entropy.artifact.WithPayload(payload)
	return len(payload), nil
}

func (entropy *Entropy) Read(payload []byte) (int, error) {
	state := datura.Acquire("entropy-state", datura.APPJSON)
	state.Inspect("statistic", "entropy", "Read()", "p")

	if _, err := state.Write(entropy.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"entropy: sample is non-finite",
			nil,
		))
	}

	history := datura.Peek[[]float64](entropy.artifact, "history")
	history = append(history, sample)
	entropy.artifact.Poke(history, "history")

	probabilities, ok := entropy.normalizedProbabilities(history)

	if !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"entropy: unable to compute normalized probabilities",
			EntropyError(EntropyErrorNonFiniteMass),
		))
	}

	value := stat.Entropy(probabilities)
	state.MergeOutput("value", value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (entropy *Entropy) Close() error {
	return nil
}

func (entropy *Entropy) normalizedProbabilities(values []float64) ([]float64, bool) {
	floor := entropy.probabilityFloor(values)

	if floor < 0 {
		return nil, false
	}

	total := 0.0
	masses := make([]float64, len(values))

	for index, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, false
		}

		if value < floor {
			return nil, false
		}

		masses[index] = value
		total += value
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
	floor := datura.Peek[float64](entropy.artifact, "config", "floor")

	if floor > 0 {
		return floor
	}

	total := floats.Sum(values)
	scale := total / float64(len(values))

	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return -1
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
