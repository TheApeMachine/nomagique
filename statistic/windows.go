package statistic

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
Windows resolves short, long, return-lag, and target-long counts from history.
*/
type Windows struct {
	artifact *datura.Artifact
}

/*
NewWindows returns a window-resolution stage wired from config attributes on the artifact.
*/
func NewWindows(artifact *datura.Artifact) *Windows {
	return &Windows{
		artifact: artifact,
	}
}

func (windows *Windows) Read(payload []byte) (int, error) {
	state := datura.Acquire("windows-state", datura.APPJSON)

	if _, err := state.Write(windows.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"windows: state write failed",
			err,
		))
	}

	defer state.Release()

	history := datura.Peek[[]float64](state, "history")
	shortHint := int(datura.Peek[float64](windows.artifact, "config", "shortHint"))
	longHint := int(datura.Peek[float64](windows.artifact, "config", "longHint"))
	lagHint := int(datura.Peek[float64](windows.artifact, "config", "returnLagHint"))
	sampleCount := len(history)
	shortWindow := 0
	longWindow := 0

	if shortHint > 0 {
		shortWindow = shortHint
	}

	if longHint > 0 {
		longWindow = longHint
	}

	if shortWindow > 0 && longWindow > 0 {
		returnLag := lagHint

		if lagHint <= 0 {
			returnLag = max(1, int(math.Ceil(math.Sqrt(float64(longWindow)))))

			if longWindow > 1 {
				returnLag = min(returnLag, longWindow-1)
			}
		}

		state.MergeOutput("shortWindow", float64(shortWindow))
		state.MergeOutput("longWindow", float64(longWindow))
		state.MergeOutput("returnLag", float64(returnLag))
		state.MergeOutput("targetLong", float64(longWindow))
		state.Poke("output", "root")
		state.Poke([]string{
			"shortWindow", "longWindow", "returnLag", "targetLong",
		}, "inputs")

		return state.Read(payload)
	}

	if sampleCount <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"windows: history or explicit hints required",
			nil,
		))
	}

	if shortWindow <= 0 {
		shortWindow = max(1, int(math.Ceil(math.Sqrt(float64(sampleCount)))))
	}

	if longWindow <= 0 {
		spread := 0.0

		if sampleCount >= 2 {
			mean := stat.Mean(history, nil)

			if mean > 0 && !math.IsNaN(mean) && !math.IsInf(mean, 0) {
				std := stat.StdDev(history, nil)

				if std > 0 && !math.IsNaN(std) && !math.IsInf(std, 0) {
					spread = std / math.Abs(mean)
				}
			}
		}

		longWindow = int(math.Ceil(float64(shortWindow) * (1.0 + spread)))

		if longWindow < sampleCount {
			longWindow = sampleCount
		}

		if longWindow <= shortWindow {
			longWindow = shortWindow + 1
		}

		if longWindow > sampleCount {
			longWindow = sampleCount
		}
	}

	targetLong := longWindow

	if longHint <= 0 {
		resolvedShort := shortHint

		if resolvedShort <= 0 {
			resolvedShort = max(1, int(math.Ceil(math.Sqrt(float64(sampleCount)))))
		}

		spread := 0.0

		if sampleCount >= 2 {
			mean := stat.Mean(history, nil)

			if mean > 0 && !math.IsNaN(mean) && !math.IsInf(mean, 0) {
				std := stat.StdDev(history, nil)

				if std > 0 && !math.IsNaN(std) && !math.IsInf(std, 0) {
					spread = std / math.Abs(mean)
				}
			}
		}

		targetLong = int(math.Ceil(float64(resolvedShort) * (1.0 + spread)))

		if targetLong < sampleCount {
			targetLong = sampleCount
		}

		if targetLong <= resolvedShort {
			targetLong = resolvedShort + 1
		}

		if targetLong > sampleCount && sampleCount > resolvedShort {
			targetLong = sampleCount
		}
	}

	returnLag := lagHint

	if lagHint <= 0 {
		returnLag = max(1, int(math.Ceil(math.Sqrt(float64(longWindow)))))

		if longWindow > 1 {
			returnLag = min(returnLag, longWindow-1)
		}
	}

	state.MergeOutput("shortWindow", float64(shortWindow))
	state.MergeOutput("longWindow", float64(longWindow))
	state.MergeOutput("returnLag", float64(returnLag))
	state.MergeOutput("targetLong", float64(targetLong))
	state.Poke("output", "root")
	state.Poke([]string{
		"shortWindow", "longWindow", "returnLag", "targetLong",
	}, "inputs")

	return state.Read(payload)
}

func (windows *Windows) Write(payload []byte) (int, error) {
	windows.artifact.WithPayload(payload)
	return len(payload), nil
}

func (windows *Windows) Close() error {
	return nil
}

/*
ResolveWindows runs the Windows stage via FlipFlop for imperative call sites.
ponytail: stages that cannot nest pipeline during Read use this instead of duplicating flipFlop wiring.
*/
func ResolveWindows(
	history []float64,
	shortHint, longHint int,
) (shortWindow, longWindow int, err error) {
	config := datura.Acquire("windows-resolve", datura.APPJSON)

	if shortHint > 0 {
		config.Poke(float64(shortHint), "config", "shortHint")
	}

	if longHint > 0 {
		config.Poke(float64(longHint), "config", "longHint")
	}

	stage := NewWindows(config)
	wire := datura.Acquire("windows-resolve-wire", datura.APPJSON)
	wire.Poke(history, "history")

	if flipErr := transport.NewFlipFlop(wire, stage); flipErr != nil {
		wire.Release()
		stage.Close()

		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"windows: flipFlop failed",
			flipErr,
		))
	}

	shortWindow = int(datura.Peek[float64](wire, "output", "shortWindow"))
	longWindow = int(datura.Peek[float64](wire, "output", "longWindow"))
	wire.Release()
	stage.Close()

	return shortWindow, longWindow, nil
}
