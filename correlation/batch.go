package correlation

import "time"

/*
DefaultMaxInterval bounds how far apart consecutive samples may sit before their
return interval is dropped from Hayashi-Yoshida. It is an estimator validity bound.
*/
const DefaultMaxInterval = 5 * time.Minute

/*
BetweenSamples estimates Hayashi-Yoshida correlation for two sample series.
*/
func BetweenSamples(
	left, right []Sample,
	maxInterval time.Duration,
) (float64, bool) {
	if maxInterval <= 0 {
		maxInterval = DefaultMaxInterval
	}

	return hayashiYoshidaCorrelation(left, right, maxInterval)
}

/*
ShiftSamplesInto writes samples with timestamps moved by offset into destination.
*/
func ShiftSamplesInto(
	destination []Sample,
	samples []Sample,
	offset time.Duration,
) []Sample {
	if len(samples) == 0 {
		return destination[:0]
	}

	if cap(destination) < len(samples) {
		destination = make([]Sample, len(samples))
	} else {
		destination = destination[:len(samples)]
	}

	if offset == 0 {
		copy(destination, samples)

		return destination
	}

	for index := range samples {
		destination[index] = Sample{
			At:    samples[index].At.Add(offset),
			Value: samples[index].Value,
		}
	}

	return destination
}
