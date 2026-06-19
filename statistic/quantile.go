package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
Quantile computes a sample quantile over retained history.
*/
type Quantile struct {
	artifact   *datura.Artifact
	percentile float64
	kind       stat.CumulantKind
}

/*
NewQuantile creates a quantile stage at percentile in [0, 1].
*/
func NewQuantile(
	percentile float64,
	kind stat.CumulantKind,
) *Quantile {
	return &Quantile{
		artifact:   datura.Acquire("quantile", datura.APPJSON),
		percentile: percentile,
		kind:       kind,
	}
}

func (quantile *Quantile) Write(p []byte) (int, error) {
	return quantile.artifact.Write(p)
}

func (quantile *Quantile) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](quantile.artifact, "sample")
	history := datura.Peek[[]float64](quantile.artifact, "history")

	if !math.IsNaN(sample) && !math.IsInf(sample, 0) {
		history = append(history, sample)
		quantile.artifact.Poke(history, "history")
	}

	if len(history) == 0 {
		return quantile.artifact.Read(p)
	}

	sorted := append([]float64(nil), history...)
	sort.Float64s(sorted)

	value := quantile.quantileOf(sorted, nil)
	quantile.artifact.Poke(datura.Map[float64]{"value": value}, "output")

	return quantile.artifact.Read(p)
}

func (quantile *Quantile) Close() error {
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

/*
QuantileOf returns the sample quantile with linear interpolation.
*/
func QuantileOf(percentile float64, values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	return stat.Quantile(percentile, stat.LinInterp, sorted, nil)
}

type QuantileErrorType string

const (
	QuantileErrorWeightLengthMismatch QuantileErrorType = "require equal weight length"
)

type QuantileError string

func (quantileError QuantileError) Error() string {
	return string(quantileError)
}
