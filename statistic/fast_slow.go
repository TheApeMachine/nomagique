package statistic

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
FastSlow compares trailing fast-window mean rate to the preceding slow window.
*/
type FastSlow struct {
	artifact *datura.Artifact
}

/*
NewFastSlow returns a breakout-ratio stage wired from config attributes on the artifact.
*/
func NewFastSlow(artifact *datura.Artifact) *FastSlow {
	return &FastSlow{
		artifact: artifact,
	}
}

func (fastSlow *FastSlow) Read(payload []byte) (int, error) {
	state := datura.Acquire("fast-slow-state", datura.APPJSON)

	if _, err := state.Write(fastSlow.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: state write failed",
			err,
		))
	}

	rootKey := datura.Peek[string](state, "root")

	if rootKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: root required",
			nil,
		))
	}

	inputs := datura.Peek[[]string](state, "inputs")

	if len(inputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: inputs required",
			nil,
		))
	}

	configInput := datura.Peek[string](fastSlow.artifact, "input")

	if configInput == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: input required",
			nil,
		))
	}

	outputKey := datura.Peek[string](fastSlow.artifact, "outputKey")

	if outputKey == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: outputKey required",
			nil,
		))
	}

	var sample float64
	found := false

	for index, input := range inputs {
		if input != configInput {
			continue
		}

		if rootKey == "features" {
			features := datura.Peek[[]float64](state, rootKey)

			if index >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"fast-slow: feature index out of range",
					nil,
				))
			}

			sample = features[index]
		}

		if rootKey != "features" {
			sample = datura.Peek[float64](state, rootKey, input)
		}

		found = true
	}

	if !found {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: input not in inputs",
			nil,
		))
	}

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
	windowsConfig := datura.Acquire("fast-slow-windows", datura.APPJSON)

	if fastHint > 0 {
		windowsConfig.Poke(float64(fastHint), "config", "shortHint")
	}

	windowsStage := NewWindows(windowsConfig)
	wire := datura.Acquire("fast-slow-windows-wire", datura.APPJSON)
	wire.Poke(history, "history")
	packed, packErr := wire.Message().MarshalPacked()
	wire.Release()

	if packErr != nil {
		windowsConfig.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: windows wire pack failed",
			packErr,
		))
	}

	if _, err := windowsStage.Write(packed); err != nil {
		windowsConfig.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: windows write failed",
			err,
		))
	}

	windowsConfig.Release()
	buffer := make([]byte, max(len(packed)*2, len(packed)+1))
	readCount, err := windowsStage.Read(buffer)

	if err != nil && err != io.ErrShortBuffer && err != io.EOF {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: unable to resolve fast window",
			err,
		))
	}

	windowsOut := datura.Acquire("fast-slow-windows-out", datura.APPJSON)

	if _, err := windowsOut.Write(buffer[:readCount]); err != nil {
		windowsOut.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: windows output read failed",
			err,
		))
	}

	fastWindow := int(datura.Peek[float64](windowsOut, "output", "shortWindow"))
	windowsOut.Release()
	windowsStage.Close()

	if len(history) <= fastWindow {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: insufficient samples for fast window",
			nil,
		))
	}

	sampleCount := len(history)
	slowCount := sampleCount - fastWindow
	recentRate := stat.Mean(history[sampleCount-fastWindow:], nil)

	if slowCount <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: insufficient samples for slow window",
			nil,
		))
	}

	olderRate := stat.Mean(history[:slowCount], nil)
	value := recentRate / olderRate

	if invert {
		if recentRate <= 0 || olderRate <= 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"fast-slow: rate is non-positive",
				nil,
			))
		}

		value = olderRate / recentRate
	}

	if !invert && olderRate <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"fast-slow: older rate is non-positive",
			nil,
		))
	}

	state.MergeOutput(outputKey, value)
	state.Poke("output", "root")
	state.Poke([]string{outputKey}, "inputs")

	return state.Read(payload)
}

func (fastSlow *FastSlow) Write(payload []byte) (int, error) {
	fastSlow.artifact.WithPayload(payload)
	return len(payload), nil
}

func (fastSlow *FastSlow) Close() error {
	return nil
}
