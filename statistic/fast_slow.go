package statistic

import (
	"fmt"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
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
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: sample must be finite and non-negative",
			nil,
		))
	}

	history := datura.Peek[[]float64](fastSlow.artifact, "history")
	history = append(history, sample)
	fastSlow.artifact.Poke(history, "history")

	fastHint := int(datura.Peek[float64](fastSlow.artifact, "config", "fastWindow"))
	invert := datura.Peek[float64](fastSlow.artifact, "config", "invert") > 0
	fastWindow, _, err := RollingWindows(history, fastHint, 0)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: unable to resolve fast window",
			err,
		))
	}

	if len(history) <= fastWindow {
		return 0, nil
	}

	value, err := fastSlowRate(history, fastWindow, invert)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: unable to compute rate",
			err,
		))
	}

	state.MergeOutput("value", value)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(payload)
}

func (fastSlow *FastSlow) Close() error {
	return nil
}

func fastSlowRate(samples []float64, fastWindow int, invert bool) (float64, error) {
	sampleCount := len(samples)

	if fastWindow <= 0 {
		var err error

		fastWindow, _, err = RollingWindows(samples, 0, 0)

		if err != nil {
			return 0, err
		}
	}

	if sampleCount <= fastWindow {
		return 0, fmt.Errorf("statistic: insufficient samples for fast window")
	}

	slowCount := sampleCount - fastWindow
	recentRate := stat.Mean(samples[sampleCount-fastWindow:], nil)

	if slowCount <= 0 {
		return 0, fmt.Errorf("statistic: insufficient samples for slow window")
	}

	olderRate := stat.Mean(samples[:slowCount], nil)

	if invert {
		if recentRate <= 0 {
			return 0, fmt.Errorf("statistic: recent rate is non-positive")
		}

		if olderRate <= 0 {
			return 0, fmt.Errorf("statistic: older rate is non-positive")
		}

		return olderRate / recentRate, nil
	}

	if olderRate <= 0 {
		return 0, fmt.Errorf("statistic: older rate is non-positive")
	}

	return recentRate / olderRate, nil
}

func FastSlowRate(samples []float64, fastWindow int) (float64, bool) {
	value, err := fastSlowRate(samples, fastWindow, false)

	return value, err == nil
}

func InvertedFastSlowRate(samples []float64, fastWindow int) (float64, bool) {
	value, err := fastSlowRate(samples, fastWindow, true)

	return value, err == nil
}
