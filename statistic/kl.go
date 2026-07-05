package statistic

import (
	"math"

	"github.com/theapemachine/errnie"
)

/*
KLDivergence computes KL divergence between paired sample streams.
*/
type KLDivergence struct {
	samples []float64
	paired  []float64
}

/*
NewKLDivergence returns a typed KL-divergence accumulator.
*/
func NewKLDivergence() *KLDivergence {
	return &KLDivergence{}
}

/*
Measure adds one paired observation and returns divergence when ready.
*/
func (klDivergence *KLDivergence) Measure(sample PairSample) (ScalarOutput, error) {
	if err := finiteStatistic("kl-divergence", sample.Sample); err != nil {
		return ScalarOutput{}, err
	}

	if err := finiteStatistic("kl-divergence", sample.Paired); err != nil {
		return ScalarOutput{}, err
	}

	if sample.Sample < 0 || sample.Paired < 0 {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: samples must be non-negative",
			nil,
		))
	}

	klDivergence.samples = append(klDivergence.samples, sample.Sample)
	klDivergence.paired = append(klDivergence.paired, sample.Paired)

	if len(klDivergence.samples) < 2 {
		return ScalarOutput{
			Ready: false,
			Count: len(klDivergence.samples),
		}, nil
	}

	value, err := klValue(klDivergence.samples, klDivergence.paired)
	if err != nil {
		return ScalarOutput{}, err
	}

	return ScalarOutput{
		Value: value,
		Ready: true,
		Count: len(klDivergence.samples),
	}, nil
}

func klValue(samples, paired []float64) (float64, error) {
	totalSample := 0.0
	totalPaired := 0.0

	for index := range samples {
		totalSample += samples[index]
		totalPaired += paired[index]
	}

	if totalSample <= 0 || totalPaired <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: totals must be positive",
			nil,
		))
	}

	probabilityFloor := min(totalSample, totalPaired) / float64(len(samples))

	if probabilityFloor <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"kl-divergence: probability floor is non-positive",
			nil,
		))
	}

	divergence := 0.0

	for index := range samples {
		probabilitySample := max(samples[index], probabilityFloor) / totalSample
		probabilityPaired := max(paired[index], probabilityFloor) / totalPaired
		divergence += probabilitySample * math.Log(probabilitySample/probabilityPaired)
	}

	return divergence, nil
}
