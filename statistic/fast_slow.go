package statistic

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/theapemachine/datura"
	"gonum.org/v1/gonum/stat"
)

/*
FastSlowRatio compares the mean rate in the trailing fast window to the mean
rate in the preceding slow window.
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
type FastSlow struct {
	artifact *datura.Artifact
	ratio    *FastSlowRatio
}

/*
NewFastSlow creates a breakout-ratio stage.
*/
func NewFastSlow(fastWindow int, epsilon float64) *FastSlow {
	return &FastSlow{
		artifact: datura.Acquire("fast_slow", datura.Artifact_Type_json),
		ratio:    NewFastSlowRatio(fastWindow, epsilon),
	}
}

/*
NewInvertedFastSlow creates a compression-ratio stage.
*/
func NewInvertedFastSlow(fastWindow int, epsilon float64) *FastSlow {
	return &FastSlow{
		artifact: datura.Acquire("fast_slow", datura.Artifact_Type_json),
		ratio:    NewInvertedFastSlowRatio(fastWindow, epsilon),
	}
}

func (fastSlow *FastSlow) Write(p []byte) (int, error) {
	return fastSlow.artifact.Write(p)
}

func (fastSlow *FastSlow) Read(p []byte) (int, error) {
	payload, err := fastSlow.artifact.Payload()

	if err == nil && len(payload) >= 8 && len(payload)%8 == 0 {
		count := len(payload) / 8
		stream := make([]float64, count)

		for index := range count {
			offset := index * 8
			stream[index] = math.Float64frombits(binary.BigEndian.Uint64(payload[offset : offset+8]))
		}

		value, computeErr := fastSlow.ratio.Next(0, stream...)

		if computeErr == nil {
			putFloat64Payload(&fastSlow.artifact, "fast_slow", value)
		}
	}

	return fastSlow.artifact.Read(p)
}

func (fastSlow *FastSlow) Close() error {
	return nil
}

func (fastSlow *FastSlow) Reset() error {
	return nil
}
