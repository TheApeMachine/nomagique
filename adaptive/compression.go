package adaptive

/*
Compression scores how far below the running baseline the current sample sits.
*/
type Compression struct {
	baselines map[string]float64
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
		baselines: map[string]float64{},
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
*/
func (compression *Compression) MeasureSeries(series string, sample float64) (CompressionOutput, error) {
	if err := finiteAdaptive("compression", sample); err != nil {
		return CompressionOutput{}, err
	}

	if series == "" {
		series = "default"
	}

	baseline := compression.baselines[series]

	if baseline <= 0 {
		compression.baselines[series] = sample

		return CompressionOutput{
			Ready: true,
			Count: 1,
		}, nil
	}

	if sample > baseline {
		compression.baselines[series] = sample

		return CompressionOutput{
			Ready: true,
			Count: 1,
		}, nil
	}

	return CompressionOutput{
		Value: (baseline - sample) / baseline,
		Ready: true,
		Count: 1,
	}, nil
}
