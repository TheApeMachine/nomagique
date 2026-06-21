package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
FastSlow compares trailing fast-window mean rate to the preceding slow window.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type FastSlow struct {
	artifact *datura.Artifact
}

/*
NewFastSlow returns a breakout-ratio stage wired from config attributes on the artifact.
*/
func NewFastSlow(artifact *datura.Artifact) *FastSlow {
	artifact.Inspect("statistic", "fast-slow", "NewFastSlow()")

	return &FastSlow{
		artifact: artifact,
	}
}

/*
NewInvertedFastSlow returns a compression-ratio stage wired from config attributes on the artifact.
*/
func NewInvertedFastSlow(artifact *datura.Artifact) *FastSlow {
	artifact.Poke(float64(1), "config", "invert")

	return NewFastSlow(artifact)
}

func (fastSlow *FastSlow) Write(payload []byte) (int, error) {
	fastSlow.artifact.WithPayload(payload)
	return len(payload), nil
}

func (fastSlow *FastSlow) Read(payload []byte) (int, error) {
	state := datura.Acquire("fast-slow-state", datura.APPJSON)
	state.Inspect("statistic", "fast-slow", "Read()", "p")

	if _, err := state.Write(fastSlow.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	sample := datura.Peek[float64](state, "sample")

	if math.IsNaN(sample) || math.IsInf(sample, 0) || sample < 0 {
		return state.Read(payload)
	}

	history := datura.Peek[[]float64](fastSlow.artifact, "history")
	history = append(history, sample)
	fastSlow.artifact.Poke(history, "history")

	fastWindow := int(datura.Peek[float64](fastSlow.artifact, "config", "fastWindow"))
	epsilon := datura.Peek[float64](fastSlow.artifact, "config", "epsilon")
	invert := datura.Peek[float64](fastSlow.artifact, "config", "invert") > 0

	if fastWindow <= 0 {
		fastWindow = 3
	}

	value := 1.0

	if invert {
		value = invertedFastSlowRate(history, fastWindow, epsilon)
	}

	if !invert {
		value = fastSlowRate(history, fastWindow, epsilon)
	}

	state.MergeOutput("value", value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
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
