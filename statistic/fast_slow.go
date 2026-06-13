package statistic

import (
	"fmt"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/core"
)

/*
FastSlowRatio compares the mean rate in the trailing fast window to the mean
rate in the preceding slow window. A zero slow baseline is smoothed by
recentRate * epsilon so breakouts after silence produce a high ratio.
*/
type FastSlowRatio struct {
	fastWindow int
	epsilon    float64
	invert     bool
}

func NewFastSlowRatio(fastWindow int, epsilon float64) *FastSlowRatio {
	if fastWindow <= 0 {
		fastWindow = 3
	}

	return &FastSlowRatio{
		fastWindow: fastWindow,
		epsilon:    epsilon,
	}
}

/*
NewInvertedFastSlowRatio returns older/recent for compression-style metrics.
*/
func NewInvertedFastSlowRatio(fastWindow int, epsilon float64) *FastSlowRatio {
	ratio := NewFastSlowRatio(fastWindow, epsilon)
	ratio.invert = true

	return ratio
}

func (ratio *FastSlowRatio) Next(
	out float64, values ...float64,
) (float64, error) {
	_ = out

	for _, sample := range values {
		if sample < 0 {
			return 0, fmt.Errorf("statistic: FastSlowRatio negative sample")
		}
	}

	if ratio.invert {
		return InvertedFastSlowRate(values, ratio.fastWindow, ratio.epsilon), nil
	}

	return FastSlowRate(values, ratio.fastWindow, ratio.epsilon), nil
}

func FastSlowRate(samples []float64, fastWindow int, epsilon float64) float64 {
	sampleCount := len(samples)

	if fastWindow <= 0 {
		fastWindow = 3
	}

	if sampleCount <= fastWindow {
		return 1.0
	}

	slowCount := sampleCount - fastWindow
	recentRate := float64(NewMean(nil).Observe(nomagique.Numbers(samples[sampleCount-fastWindow:]...)...))

	if slowCount <= 0 {
		return 1.0
	}

	olderRate := float64(NewMean(nil).Observe(nomagique.Numbers(samples[:slowCount]...)...))

	if olderRate <= 0 {
		olderRate = recentRate * epsilon

		if olderRate <= 0 {
			return 1.0
		}
	}

	return recentRate / olderRate
}

func InvertedFastSlowRate(samples []float64, fastWindow int, epsilon float64) float64 {
	sampleCount := len(samples)

	if fastWindow <= 0 {
		fastWindow = 3
	}

	if sampleCount <= fastWindow {
		return 0.0
	}

	slowCount := sampleCount - fastWindow
	recentRate := float64(NewMean(nil).Observe(nomagique.Numbers(samples[sampleCount-fastWindow:]...)...))

	if recentRate <= 0 {
		return 0.0
	}

	if slowCount <= 0 {
		return 0.0
	}

	olderRate := float64(NewMean(nil).Observe(nomagique.Numbers(samples[:slowCount]...)...))

	if olderRate <= 0 {
		olderRate = recentRate * epsilon
	}

	return olderRate / recentRate
}

/*
FastSlow compares trailing fast-window mean rate to the preceding slow window.
*/
type FastSlow struct {
	stream core.Numbers
	ratio  *FastSlowRatio
}

/*
NewFastSlow creates a breakout-ratio dynamic over a configured stream.
*/
func NewFastSlow(stream core.Numbers, fastWindow int, epsilon float64) *FastSlow {
	return &FastSlow{
		stream: stream,
		ratio:  NewFastSlowRatio(fastWindow, epsilon),
	}
}

/*
NewInvertedFastSlow creates a compression-ratio dynamic over a configured stream.
*/
func NewInvertedFastSlow(stream core.Numbers, fastWindow int, epsilon float64) *FastSlow {
	return &FastSlow{
		stream: stream,
		ratio:  NewInvertedFastSlowRatio(fastWindow, epsilon),
	}
}

/*
Observe returns the configured fast/slow ratio for the stream history.
*/
func (fastSlow *FastSlow) Observe(_ ...core.Number) core.Float64 {
	samples := nomagique.Samples(fastSlow.stream)
	value, err := fastSlow.ratio.Next(0, samples...)

	if err != nil {
		return 0
	}

	return core.Float64(value)
}

/*
Reset clears derived state.
*/
func (fastSlow *FastSlow) Reset() error {
	fastSlow.stream = nil
	return nil
}
