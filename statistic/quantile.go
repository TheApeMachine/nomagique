package statistic

import (
	"sort"

	"gonum.org/v1/gonum/stat"
)

/*
QuantileConfig configures a typed quantile accumulator.
*/
type QuantileConfig struct {
	Percentile float64
	Kind       stat.CumulantKind
}

/*
Quantile computes a sample quantile over retained history.
*/
type Quantile struct {
	config  QuantileConfig
	history []float64
}

/*
NewQuantile returns a typed quantile accumulator.
*/
func NewQuantile(configs ...QuantileConfig) *Quantile {
	config := QuantileConfig{
		Percentile: 0.5,
		Kind:       stat.LinInterp,
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
	sorted := append([]float64(nil), quantile.history...)
	sort.Float64s(sorted)

	value := sorted[0]

	if quantile.config.Percentile > 0 && quantile.config.Percentile < 1 {
		value = stat.Quantile(quantile.config.Percentile, quantile.config.Kind, sorted, nil)
	}

	if quantile.config.Percentile >= 1 {
		value = sorted[len(sorted)-1]
	}

	return ScalarOutput{
		Value: value,
		Ready: true,
		Count: len(quantile.history),
	}, nil
}
