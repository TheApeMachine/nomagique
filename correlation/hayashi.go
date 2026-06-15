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
		artifact:    datura.Acquire("hayashi", datura.Artifact_Type_json),
		weights:     weights,
		maxInterval: maxInterval,
	}
}

func (hayashi *HayashiYoshida) Write(p []byte) (int, error) {
	return hayashi.artifact.Write(p)
}

func (hayashi *HayashiYoshida) Read(p []byte) (int, error) {
	values := float64Batch(hayashi.artifact)
	count := len(values)

	if count >= 4 && count%2 == 0 {
		half := count / 2
		left, leftOK := samplesFromScalars(values[:half])
		right, rightOK := samplesFromScalars(values[half:])

		if leftOK && rightOK {
			correlation, ok := hayashiYoshidaCorrelation(left, right, hayashi.maxInterval)

			if ok {
				putFloat64Payload(&hayashi.artifact, "hayashi", correlation)

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

	putFloat64Payload(&hayashi.artifact, "hayashi", 0)

	return hayashi.artifact.Read(p)
}

func (hayashi *HayashiYoshida) Close() error {
	return nil
}

func (hayashi *HayashiYoshida) Reset() error {
	hayashi.weights = nil

	return nil
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
