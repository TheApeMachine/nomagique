package statistic

import (
	"fmt"

	"github.com/theapemachine/nomagique/core"
	"gonum.org/v1/gonum/stat"
)

/*
FastSlowRatio compares the mean rate in the trailing fast window to the mean
rate in the preceding slow window.

A spike after a quiet baseline yields a high ratio without a hard cap; a zero
slow baseline is smoothed by recentRate * epsilon.
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
	recentRate := stat.Mean(samples[sampleCount-fastWindow:], nil)

	if slowCount <= 0 {
		return 1.0
	}

	olderRate := stat.Mean(samples[:slowCount], nil)

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
	recentRate := stat.Mean(samples[sampleCount-fastWindow:], nil)

	if recentRate <= 0 {
		return 0.0
	}

	if slowCount <= 0 {
		return 0.0
	}

	olderRate := stat.Mean(samples[:slowCount], nil)

	if olderRate <= 0 {
		olderRate = recentRate * epsilon
	}

	return olderRate / recentRate
}

/*
FastSlow compares trailing fast-window mean rate to the preceding slow window.
*/
type FastSlow[T ~float64] struct {
	stream []float64
	ratio  *FastSlowRatio
	output core.Scalar[T]
}

/*
NewFastSlow creates a breakout-ratio stage over a configured stream.
*/
func NewFastSlow[T ~float64](
	stream []float64, fastWindow int, epsilon float64,
) *FastSlow[T] {
	return &FastSlow[T]{
		stream: stream,
		ratio:  NewFastSlowRatio(fastWindow, epsilon),
	}
}

/*
NewInvertedFastSlow creates a compression-ratio stage over a configured stream.
*/
func NewInvertedFastSlow[T ~float64](
	stream []float64, fastWindow int, epsilon float64,
) *FastSlow[T] {
	return &FastSlow[T]{
		stream: stream,
		ratio:  NewInvertedFastSlowRatio(fastWindow, epsilon),
	}
}

/*
Observe returns the configured fast/slow ratio for the stream history.
*/
func (fastSlow *FastSlow[T]) Observe(_ ...core.Number[T]) core.Scalar[T] {
	value, err := fastSlow.ratio.Next(0, fastSlow.stream...)

	if err != nil {
		return fastSlow.output
	}

	fastSlow.output = core.Scalar[T](T(value))

	return fastSlow.output
}

/*
Reset clears derived state.
*/
func (fastSlow *FastSlow[T]) Reset() error {
	fastSlow.stream = nil
	fastSlow.output = core.Scalar[T](0)

	return nil
}
