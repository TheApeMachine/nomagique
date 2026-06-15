package statistic

import (
	"encoding/binary"
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Quantile computes a sample quantile using gonum's stat.Quantile interpolation.
*/
type Quantile struct {
	artifact   *datura.Artifact
	percentile float64
	kind       stat.CumulantKind
	weights    []float64
}

/*
NewQuantile creates a quantile stage at percentile in [0, 1].
*/
func NewQuantile(
	percentile float64,
	kind stat.CumulantKind,
	weights []float64,
) *Quantile {
	return &Quantile{
		artifact:   datura.Acquire("quantile", datura.Artifact_Type_json),
		percentile: percentile,
		kind:       kind,
		weights:    weights,
	}
}

func (quantile *Quantile) Write(p []byte) (int, error) {
	return quantile.artifact.Write(p)
}

func (quantile *Quantile) Read(p []byte) (int, error) {
	payload, err := quantile.artifact.Payload()

	if err != nil || len(payload) < 8 || len(payload)%8 != 0 {
		return quantile.artifact.Read(p)
	}

	count := len(payload) / 8
	values := make([]float64, count)

	for index := range count {
		offset := index * 8
		values[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
	}

	if len(values) == 0 {
		return quantile.artifact.Read(p)
	}

	weights := quantile.weights

	if len(weights) != 0 {
		if len(weights) != len(values) {
			errnie.Err(
				errnie.Validation, "unable to compute quantile",
				QuantileError(QuantileErrorWeightLengthMismatch),
			)

			return quantile.artifact.Read(p)
		}

		sortedValues, sortedWeights, ok := sortWeightedSamples(values, weights)

		if !ok {
			putFloat64Payload(&quantile.artifact, "quantile", math.NaN())

			return quantile.artifact.Read(p)
		}

		putFloat64Payload(&quantile.artifact, "quantile", quantile.quantileOf(sortedValues, sortedWeights))

		return quantile.artifact.Read(p)
	}

	sort.Float64s(values)

	putFloat64Payload(&quantile.artifact, "quantile", quantile.quantileOf(values, nil))

	return quantile.artifact.Read(p)
}

func (quantile *Quantile) Close() error {
	return nil
}

func (quantile *Quantile) Reset() error {
	quantile.weights = nil

	return nil
}

func (quantile *Quantile) quantileOf(
	sorted []float64, weights []float64,
) float64 {
	if quantile.percentile <= 0 {
		return sorted[0]
	}

	if quantile.percentile >= 1 {
		return sorted[len(sorted)-1]
	}

	if weights == nil {
		for _, value := range sorted {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return math.NaN()
			}
		}

		return stat.Quantile(quantile.percentile, quantile.kind, sorted, nil)
	}

	return stat.Quantile(quantile.percentile, quantile.kind, sorted, weights)
}

func sortWeightedSamples(values, weights []float64) ([]float64, []float64, bool) {
	for index, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, nil, false
		}

		weight := weights[index]

		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
			return nil, nil, false
		}
	}

	sortedValues := make([]float64, len(values))
	copy(sortedValues, values)
	sortedWeights := make([]float64, len(weights))
	copy(sortedWeights, weights)

	stat.SortWeighted(sortedValues, sortedWeights)

	return sortedValues, sortedWeights, true
}

type QuantileErrorType string

const (
	QuantileErrorWeightLengthMismatch QuantileErrorType = "require equal weight length"
)

type QuantileError string

func (quantileError QuantileError) Error() string {
	return string(quantileError)
}
