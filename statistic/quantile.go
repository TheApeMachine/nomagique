package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Quantile computes a sample quantile over retained history.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type Quantile struct {
	artifact *datura.Artifact
}

/*
NewQuantile returns a quantile stage wired from config attributes on the artifact.
Percentile and cumulant kind live under config.percentile and config.kind.
*/
func NewQuantile(artifact *datura.Artifact) *Quantile {
	artifact.Inspect("statistic", "quantile", "NewQuantile()")

	return &Quantile{
		artifact: artifact,
	}
}

func (quantile *Quantile) Write(payload []byte) (int, error) {
	quantile.artifact.WithPayload(payload)
	return len(payload), nil
}

func (quantile *Quantile) Read(payload []byte) (int, error) {
	state := datura.Acquire("quantile-state", datura.APPJSON)
	state.Inspect("statistic", "quantile", "Read()", "p")

	if _, err := state.Write(quantile.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quantile: sample is non-finite",
			nil,
		))
	}

	history := datura.Peek[[]float64](quantile.artifact, "history")
	history = append(history, sample)
	quantile.artifact.Poke(history, "history")

	if len(history) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quantile: insufficient history",
			nil,
		))
	}

	sorted := append([]float64(nil), history...)
	sort.Float64s(sorted)

	percentile := datura.Peek[float64](quantile.artifact, "config", "percentile")
	kind := stat.CumulantKind(int(datura.Peek[float64](quantile.artifact, "config", "kind")))
	value, ok := quantileValue(sorted, nil, percentile, kind)

	if !ok {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"quantile: history contains non-finite values",
			nil,
		))
	}

	state.MergeOutput("value", value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (quantile *Quantile) Close() error {
	return nil
}

func quantileValue(
	sorted []float64, weights []float64,
	percentile float64, kind stat.CumulantKind,
) (float64, bool) {
	if len(sorted) == 0 {
		return 0, false
	}

	if percentile <= 0 {
		return sorted[0], true
	}

	if percentile >= 1 {
		return sorted[len(sorted)-1], true
	}

	if weights == nil {
		for _, value := range sorted {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return 0, false
			}
		}

		return stat.Quantile(percentile, kind, sorted, nil), true
	}

	return stat.Quantile(percentile, kind, sorted, weights), true
}

/*
QuantileOf returns the sample quantile with linear interpolation.
*/
func QuantileOf(percentile float64, values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	for _, value := range sorted {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
	}

	return stat.Quantile(percentile, stat.LinInterp, sorted, nil), true
}

type QuantileErrorType string

const (
	QuantileErrorWeightLengthMismatch QuantileErrorType = "require equal weight length"
)

type QuantileError string

func (quantileError QuantileError) Error() string {
	return string(quantileError)
}
