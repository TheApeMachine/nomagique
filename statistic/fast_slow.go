package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
FastSlow compares trailing fast-window mean rate to the preceding slow window.
*/
type FastSlow struct {
	artifact   *datura.Artifact
	fastWindow int
	epsilon    float64
	invert     bool
}

/*
NewFastSlow creates a breakout-ratio stage.
*/
func NewFastSlow(fastWindow int, epsilon float64) *FastSlow {
	if fastWindow <= 0 {
		fastWindow = 3
	}

	return &FastSlow{
		artifact:   datura.Acquire("fast_slow", datura.APPJSON),
		fastWindow: fastWindow,
		epsilon:    epsilon,
	}
}

/*
NewInvertedFastSlow creates a compression-ratio stage.
*/
func NewInvertedFastSlow(fastWindow int, epsilon float64) *FastSlow {
	fastSlow := NewFastSlow(fastWindow, epsilon)
	fastSlow.invert = true

	return fastSlow
}

func (fastSlow *FastSlow) Write(p []byte) (int, error) {
	return fastSlow.artifact.Write(p)
}

func (fastSlow *FastSlow) Read(p []byte) (int, error) {
	sample := datura.Peek[float64](fastSlow.artifact, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) || sample < 0 {
		return fastSlow.artifact.Read(p)
	}

	history := datura.Peek[[]float64](fastSlow.artifact, "history")
	history = append(history, sample)
	fastSlow.artifact.Poke(history, "history")

	value := 1.0

	if fastSlow.invert {
		value = invertedFastSlowRate(history, fastSlow.fastWindow, fastSlow.epsilon)
	}

	if !fastSlow.invert {
		value = fastSlowRate(history, fastSlow.fastWindow, fastSlow.epsilon)
	}

	fastSlow.artifact.Poke(datura.Map[float64]{"value": value}, "output")

	return fastSlow.artifact.Read(p)
}

func (fastSlow *FastSlow) Close() error {
	return nil
}

func fastSlowRate(samples []float64, fastWindow int, epsilon float64) float64 {
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

func invertedFastSlowRate(samples []float64, fastWindow int, epsilon float64) float64 {
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

func FastSlowRate(samples []float64, fastWindow int, epsilon float64) float64 {
	return fastSlowRate(samples, fastWindow, epsilon)
}

func InvertedFastSlowRate(samples []float64, fastWindow int, epsilon float64) float64 {
	return invertedFastSlowRate(samples, fastWindow, epsilon)
}
