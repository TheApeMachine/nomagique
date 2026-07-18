package statistic

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
)

/*
QuantileConfig configures a typed quantile accumulator.
*/
type QuantileConfig struct {
	Percentile float64
}

/*
Quantile computes a sample quantile over retained history.
*/
type Quantile struct {
	config  QuantileConfig
	history []float64
}

/*
NewQuantile returns a typed quantile accumulator using the canonical
(n-1)*p linear-interpolation estimator.
*/
func NewQuantile(configs ...QuantileConfig) *Quantile {
	config := QuantileConfig{
		Percentile: 0.5,
	}

	if len(configs) > 0 {
		config = configs[0]
	}

	return &Quantile{
		config: config,
	}
}

/*
Measure adds one sample and returns the configured quantile.
*/
func (quantile *Quantile) Measure(sample float64) (ScalarOutput, error) {
	if err := finiteStatistic("quantile", sample); err != nil {
		return ScalarOutput{}, err
	}

	quantile.history = append(quantile.history, sample)

	value, err := interpolateQuantile(quantile.history, quantile.config.Percentile)

	if err != nil {
		return ScalarOutput{}, err
	}

	return ScalarOutput{
		Value: value,
		Ready: true,
		Count: len(quantile.history),
	}, nil
}

/*
interpolateQuantile evaluates Q(p) on a copy of samples with
h = (n-1)*p linear interpolation after sorting.
*/
func interpolateQuantile(samples []float64, percentile float64) (float64, error) {
	if len(samples) == 0 {
		return 0, errnie.Err(
			errnie.Validation,
			"statistic: quantile sample is empty",
			nil,
		)
	}

	if percentile < 0 || percentile > 1 || math.IsNaN(percentile) {
		return 0, errnie.Err(
			errnie.Validation,
			"statistic: quantile percentile must lie in [0, 1]",
			nil,
		)
	}

	sorted := append([]float64(nil), samples...)
	sort.Float64s(sorted)

	for _, value := range sorted {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Err(
				errnie.Validation,
				"statistic: quantile sample is non-finite",
				nil,
			)
		}
	}

	n := len(sorted)
	h := float64(n-1) * percentile
	j := int(math.Floor(h))
	g := h - float64(j)

	if j >= n-1 {
		return sorted[n-1], nil
	}

	return sorted[j] + g*(sorted[j+1]-sorted[j]), nil
}
