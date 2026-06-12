package correlation

import (
	"math"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/core"
)

/*
HayashiYoshida estimates asynchronous high-frequency correlation with a sliding
sweep over overlapping return intervals. It does not require both series to
share the same observation grid.
*/
type HayashiYoshida struct {
	weights     core.Numbers
	maxInterval time.Duration
}

/*
NewHayashiYoshida creates a Hayashi-Yoshida correlation dynamic.
When maxInterval is zero, consecutive sample spacing is not capped.
*/
func NewHayashiYoshida(
	weights core.Numbers, maxInterval time.Duration,
) *HayashiYoshida {
	return &HayashiYoshida{
		weights:     weights,
		maxInterval: maxInterval,
	}
}

/*
Observe computes Hayashi-Yoshida correlation between two encoded sample streams.
Inputs are split into equal halves; each half is a sequence of (time, value) pairs.
*/
func (hayashi *HayashiYoshida) Observe(inputs ...core.Number) core.Float64 {
	count := len(inputs)

	if count < 2 {
		errnie.Err(
			errnie.Validation, "unable to compute Hayashi-Yoshida correlation",
			HayashiError(HayashiErrorRequireAtLeastTwoInputs),
		)

		return 0
	}

	if count%2 != 0 {
		errnie.Err(
			errnie.Validation, "unable to compute Hayashi-Yoshida correlation",
			HayashiError(HayashiErrorRequireEqualLength),
		)

		return 0
	}

	half := count / 2

	left, leftOK := samplesFromNumbers(core.Numbers(inputs[:half]))

	if !leftOK {
		errnie.Err(
			errnie.Validation, "unable to compute Hayashi-Yoshida correlation",
			HayashiError(HayashiErrorRequirePairedSamples),
		)

		return 0
	}

	right, rightOK := samplesFromNumbers(core.Numbers(inputs[half:]))

	if !rightOK {
		errnie.Err(
			errnie.Validation, "unable to compute Hayashi-Yoshida correlation",
			HayashiError(HayashiErrorRequirePairedSamples),
		)

		return 0
	}

	correlation, ok := hayashiYoshidaCorrelation(left, right, hayashi.maxInterval)

	if !ok {
		return 0
	}

	return core.Float64(correlation)
}

/*
Reset clears derived state.
*/
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

func (error HayashiError) Error() string {
	return string(error)
}
