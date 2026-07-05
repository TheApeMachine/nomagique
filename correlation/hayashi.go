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

func validInterval(left, right Sample, maxInterval time.Duration) bool {
	if !left.At.Before(right.At) || left.Value <= 0 || right.Value <= 0 {
		return false
	}

	return maxInterval <= 0 || right.At.Sub(left.At) <= maxInterval
}

type HayashiErrorType string

const (
	HayashiErrorRequireAtLeastTwoInputs HayashiErrorType = "require at least two observations"
	HayashiErrorRequireEqualLength      HayashiErrorType = "require equal length time/value pairs"
	HayashiErrorRequirePairedSamples    HayashiErrorType = "require paired timestamp/value samples"
)

type HayashiError string

func (hayashiError HayashiError) Error() string {
	return string(hayashiError)
}
