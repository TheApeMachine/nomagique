package adaptive

import (
	"github.com/theapemachine/errnie"
)

/*
Compression scores how far below the running baseline the current sample sits.
It tracks an explicit initialization flag so zero or negative samples cannot
silently re-seed the baseline after the series has already started.
*/
type Compression struct {
	series map[string]*compressionSeries
}

type compressionSeries struct {
	initialized bool
	baseline    float64
	count       int
}

/*
CompressionOutput reports compression against the retained series baseline.
*/
type CompressionOutput struct {
	Value float64
	Ready bool
	Count int
}

/*
NewCompression returns a typed compression tracker.
*/
func NewCompression() *Compression {
	return &Compression{
		series: map[string]*compressionSeries{},
	}
}

/*
Measure adds one sample to the default series.
*/
func (compression *Compression) Measure(sample float64) (CompressionOutput, error) {
	return compression.MeasureSeries("default", sample)
}

/*
MeasureSeries adds one sample and compares it against the series baseline.
Compression is defined on strictly positive magnitudes so the baseline remains
a valid divisor for relative drawdown.
*/
func (compression *Compression) MeasureSeries(series string, sample float64) (CompressionOutput, error) {
	if err := finiteAdaptive("compression", sample); err != nil {
		return CompressionOutput{}, err
	}

	if sample <= 0 {
		return CompressionOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"compression: sample must be positive",
			nil,
		))
	}

	if series == "" {
		series = "default"
	}

	state := compression.series[series]

	if state == nil {
		state = &compressionSeries{}
		compression.series[series] = state
	}

	if !state.initialized {
		state.initialized = true
		state.baseline = sample
		state.count = 1

		return CompressionOutput{
			Ready: true,
			Count: state.count,
		}, nil
	}

	state.count++

	if sample > state.baseline {
		state.baseline = sample

		return CompressionOutput{
			Ready: true,
			Count: state.count,
		}, nil
	}

	return CompressionOutput{
		Value: (state.baseline - sample) / state.baseline,
		Ready: true,
		Count: state.count,
	}, nil
}
