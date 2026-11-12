package correlation

import (
	"math"
	"time"
)

/*
Sample is a time-stamped observation for asynchronous correlation.
Each input pair encodes Unix seconds at an even index and value at the next index.
*/
type Sample struct {
	At    time.Time
	Value float64
}

/*
samplesFromScalars unpacks interleaved Unix-second and value pairs from flat
buffers; non-finite scalars abort so Hayashi paths do not ingest poison times.
*/
func samplesFromScalars(values []float64) ([]Sample, bool) {
	if len(values) < 4 || len(values)%2 != 0 {
		return nil, false
	}

	samples := make([]Sample, len(values)/2)

	for index := range samples {
		pair := index * 2
		seconds := values[pair]
		value := values[pair+1]

		if math.IsNaN(seconds) || math.IsInf(seconds, 0) ||
			math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, false
		}

		wholeSeconds := int64(seconds)
		nanoseconds := int64((seconds - float64(wholeSeconds)) * float64(time.Second))

		samples[index] = Sample{
			At:    time.Unix(wholeSeconds, nanoseconds),
			Value: value,
		}
	}

	return samples, true
}

/*
hayashiYoshidaCorrelation integrates overlapping log-return products under
maxInterval so irregular async series correlate without calendar alignment.
*/
func hayashiYoshidaCorrelation(
	left, right []Sample,
	maxInterval time.Duration,
) (float64, bool) {
	if len(left) < 2 || len(right) < 2 {
		return 0, false
	}

	leftVariance := varianceSum(left, maxInterval)
	rightVariance := varianceSum(right, maxInterval)

	if leftVariance <= 0 || rightVariance <= 0 {
		return 0, false
	}

	covariance := 0.0
	rightStart := 0

	for leftIndex := 0; leftIndex < len(left)-1; leftIndex++ {
		leftStart := left[leftIndex].At
		leftEnd := left[leftIndex+1].At

		if !validInterval(left[leftIndex], left[leftIndex+1], maxInterval) {
			continue
		}

		leftReturn := math.Log(left[leftIndex+1].Value / left[leftIndex].Value)

		for rightStart < len(right)-1 {
			if !validInterval(right[rightStart], right[rightStart+1], maxInterval) ||
				!leftStart.Before(right[rightStart+1].At) {
				rightStart++
				continue
			}

			break
		}

		for rightIndex := rightStart; rightIndex < len(right)-1; rightIndex++ {
			rightIntervalStart := right[rightIndex].At

			if !rightIntervalStart.Before(leftEnd) {
				break
			}

			if !validInterval(right[rightIndex], right[rightIndex+1], maxInterval) {
				continue
			}

			covariance += leftReturn * math.Log(
				right[rightIndex+1].Value/right[rightIndex].Value,
			)
		}
	}

	denominator := math.Sqrt(leftVariance * rightVariance)

	if denominator <= 0 {
		return 0, false
	}

	correlation := covariance / denominator

	if correlation > 1 {
		return 1, true
	}

	if correlation < -1 {
		return -1, true
	}

	return correlation, true
}

/*
varianceSum accumulates realized variance from valid consecutive returns only;
invalid intervals are skipped so the denominator reflects tradeable spans.
*/
func varianceSum(samples []Sample, maxInterval time.Duration) float64 {
	if len(samples) < 2 {
		return 0
	}

	sum := 0.0

	for index := 1; index < len(samples); index++ {
		if !validInterval(samples[index-1], samples[index], maxInterval) {
			continue
		}

		ret := math.Log(samples[index].Value / samples[index-1].Value)
		sum += ret * ret
	}

	return sum
}

/*
validInterval enforces strict time order, positive prices, and optional duration
caps that keep Hayashi products from spanning stale or crossed quotes.
*/
func validInterval(left, right Sample, maxInterval time.Duration) bool {
	if !left.At.Before(right.At) || left.Value <= 0 || right.Value <= 0 {
		return false
	}

	return maxInterval <= 0 || right.At.Sub(left.At) <= maxInterval
}

/*
HayashiErrorType categorizes Hayashi input validation failures.
*/
type HayashiErrorType string

const (
	HayashiErrorRequireAtLeastTwoInputs HayashiErrorType = "require at least two observations"
	HayashiErrorRequireEqualLength      HayashiErrorType = "require equal length time/value pairs"
	HayashiErrorRequirePairedSamples    HayashiErrorType = "require paired timestamp/value samples"
)

/*
HayashiError is a string error for asynchronous correlation preconditions.
*/
type HayashiError string

var _ error = (*HayashiError)(nil)

/*
Error satisfies the error interface for HayashiError constants.
*/
func (hayashiError HayashiError) Error() string {
	return string(hayashiError)
}
