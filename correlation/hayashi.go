package correlation

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
HayashiYoshida estimates asynchronous high-frequency correlation with a sliding
sweep over overlapping return intervals. It does not require both series to
share the same observation grid.
*/
type HayashiYoshida struct {
	artifact    *datura.Artifact
	weights     []float64
	maxInterval time.Duration
}

/*
NewHayashiYoshida creates a Hayashi-Yoshida correlation dynamic.
When maxInterval is zero, consecutive sample spacing is not capped.
*/
func NewHayashiYoshida(weights []float64, maxInterval time.Duration) *HayashiYoshida {
	return &HayashiYoshida{
		artifact:    datura.Acquire("hayashi", datura.APPJSON).RetainStageAttributes(),
		weights:     weights,
		maxInterval: maxInterval,
	}
}

func (hayashi *HayashiYoshida) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](hayashi.artifact, "output") == nil

	hayashi.artifact.Clear("sample")
	hayashi.artifact.Clear("paired")
	hayashi.artifact.Clear("batch")
	hayashi.artifact.Clear("left")
	hayashi.artifact.Clear("right")

	n, err := hayashi.artifact.Write(p)

	if bootstrap {
		hayashi.artifact.Clear("output")
	}

	return n, err
}

func (hayashi *HayashiYoshida) Read(p []byte) (int, error) {
	values := datura.Peek[[]float64](hayashi.artifact, "batch")

	if len(values) == 0 {
		left := datura.Peek[[]float64](hayashi.artifact, "left")
		right := datura.Peek[[]float64](hayashi.artifact, "right")

		if len(left) > 0 || len(right) > 0 {
			values = append(append([]float64(nil), left...), right...)
		}
	}

	count := len(values)

	if count >= 4 && count%2 == 0 {
		half := count / 2
		left, leftOK := samplesFromScalars(values[:half])
		right, rightOK := samplesFromScalars(values[half:])

		if leftOK && rightOK {
			correlation, ok := hayashiYoshidaCorrelation(left, right, hayashi.maxInterval)

			if ok {
				hayashi.artifact.Poke(datura.Map[float64]{"value": correlation}, "output")

				return hayashi.artifact.Read(p)
			}
		}

		errnie.Err(
			errnie.Validation, "unable to compute Hayashi-Yoshida correlation",
			HayashiError(HayashiErrorRequirePairedSamples),
		)
	}

	if count > 0 && count < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute Hayashi-Yoshida correlation",
			HayashiError(HayashiErrorRequireAtLeastTwoInputs),
		)
	}

	if count%2 != 0 && count > 0 {
		errnie.Err(
			errnie.Validation, "unable to compute Hayashi-Yoshida correlation",
			HayashiError(HayashiErrorRequireEqualLength),
		)
	}

	hayashi.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

	return hayashi.artifact.Read(p)
}

func (hayashi *HayashiYoshida) Close() error {
	return nil
}

func (hayashi *HayashiYoshida) Reset() error {
	hayashi.weights = nil
	hayashi.artifact.Clear("output")

	return nil
}

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
	left, right []Sample, maxInterval time.Duration,
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

func validInterval(previous, current Sample, maxInterval time.Duration) bool {
	if previous.Value <= 0 || current.Value <= 0 || !previous.At.Before(current.At) {
		return false
	}

	if maxInterval <= 0 {
		return true
	}

	return current.At.Sub(previous.At) <= maxInterval
}

type HayashiErrorType string

const (
	HayashiErrorRequireAtLeastTwoInputs HayashiErrorType = "require at least two inputs"
	HayashiErrorRequireEqualLength      HayashiErrorType = "require equal length"
	HayashiErrorRequirePairedSamples    HayashiErrorType = "require paired time-value samples"
)

type HayashiError string

func (hayashiError HayashiError) Error() string {
	return string(hayashiError)
}
