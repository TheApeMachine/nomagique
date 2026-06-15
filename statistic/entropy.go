package statistic

import (
	"encoding/binary"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"
)

/*
Entropy computes Shannon entropy of a normalized mass distribution.
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
		artifact: datura.Acquire("entropy", datura.Artifact_Type_json),
		floor:    floor,
	}
}

func (entropy *Entropy) Write(p []byte) (int, error) {
	return entropy.artifact.Write(p)
}

func (entropy *Entropy) Read(p []byte) (int, error) {
	payload, err := entropy.artifact.Payload()

	if err != nil || len(payload) < 8 || len(payload)%8 != 0 {
		return entropy.artifact.Read(p)
	}

	count := len(payload) / 8
	values := make([]float64, count)

	for index := range count {
		offset := index * 8
		values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
	}

	if len(values) == 0 {
		return entropy.artifact.Read(p)
	}

	probabilities, ok := entropy.normalizedProbabilities(values)

	if !ok {
		errnie.Err(
			errnie.Validation, "unable to compute entropy",
			EntropyError(EntropyErrorNonFiniteMass),
		)

		return entropy.artifact.Read(p)
	}

	putFloat64Payload(&entropy.artifact, "entropy", stat.Entropy(probabilities))

	return entropy.artifact.Read(p)
}

func (entropy *Entropy) Close() error {
	return nil
}

func (entropy *Entropy) Reset() error {
	return nil
}

func (entropy *Entropy) normalizedProbabilities(values []float64) ([]float64, bool) {
	floor := entropy.probabilityFloor(values)
	total := 0.0

	for index := range values {
		if math.IsNaN(values[index]) || math.IsInf(values[index], 0) {
			return nil, false
		}

		mass := values[index]

		if mass < floor {
			mass = floor
		}

		values[index] = mass
		total += mass
	}

	if total <= 0 || math.IsNaN(total) || math.IsInf(total, 0) {
		return nil, false
	}

	probabilities := make([]float64, len(values))

	for index := range values {
		probabilities[index] = values[index] / total
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
