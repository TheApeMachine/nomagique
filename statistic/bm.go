package statistic

import "gonum.org/v1/gonum/stat"

/*
BivariateMomentConfig configures a typed bivariate-moment accumulator.
*/
type BivariateMomentConfig struct {
	R float64
	S float64
}

/*
BivariateMoment computes the bivariate moment between paired sample streams.
*/
type BivariateMoment struct {
	config  BivariateMomentConfig
	samples []float64
	paired  []float64
}

/*
NewBivariateMoment returns a typed bivariate-moment accumulator.
*/
func NewBivariateMoment(configs ...BivariateMomentConfig) *BivariateMoment {
	config := BivariateMomentConfig{
		R: 1,
		S: 1,
	}

	if len(configs) > 0 {
		config = configs[0]
	}

	return &BivariateMoment{
		config: config,
	}
}

/*
Measure adds one paired observation and returns the configured mixed moment.
*/
func (bivariateMoment *BivariateMoment) Measure(sample PairSample) (ScalarOutput, error) {
	if err := finiteStatistic("bivariate-moment", sample.Sample); err != nil {
		return ScalarOutput{}, err
	}

	if err := finiteStatistic("bivariate-moment", sample.Paired); err != nil {
		return ScalarOutput{}, err
	}

	bivariateMoment.samples = append(bivariateMoment.samples, sample.Sample)
	bivariateMoment.paired = append(bivariateMoment.paired, sample.Paired)

	if len(bivariateMoment.samples) < 2 {
		return ScalarOutput{
			Ready: false,
			Count: len(bivariateMoment.samples),
		}, nil
	}

	return ScalarOutput{
		Value: stat.BivariateMoment(
			bivariateMoment.config.R,
			bivariateMoment.config.S,
			bivariateMoment.samples,
			bivariateMoment.paired,
			nil,
		),
		Ready: true,
		Count: len(bivariateMoment.samples),
	}, nil
}
