package statistic

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
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
RollingWindow resolves short and long windows via the Windows stage.
ponytail: bridge for legacy Resolve() callers; upgrade path is NewWindows Write/Read at each call site.
*/
type RollingWindow struct {
	shortHint int
	longHint  int
}

/*
NewRollingWindow binds optional short and long window hints for legacy resolution.
*/
func NewRollingWindow(shortHint, longHint int) *RollingWindow {
	return &RollingWindow{
		shortHint: shortHint,
		longHint:  longHint,
	}
}

/*
Resolve derives short and long window sizes through the Windows stage.
*/
func (rolling *RollingWindow) Resolve(history []float64) (shortWindow, longWindow int, err error) {
	config := datura.Acquire("windows-bridge-config", datura.APPJSON)

	if rolling.shortHint > 0 {
		config.Poke(float64(rolling.shortHint), "config", "shortHint")
	}

	if rolling.longHint > 0 {
		config.Poke(float64(rolling.longHint), "config", "longHint")
	}

	stage := NewWindows(config)
	wire := datura.Acquire("windows-bridge-wire", datura.APPJSON)
	wire.Poke(history, "history")
	packed, packErr := wire.Message().MarshalPacked()

	if packErr != nil {
		wire.Release()
		config.Release()

		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"windows bridge: wire pack failed",
			packErr,
		))
	}

	if _, writeErr := stage.Write(packed); writeErr != nil {
		wire.Release()
		config.Release()

		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"windows bridge: write failed",
			writeErr,
		))
	}

	wire.Release()
	buffer := make([]byte, max(len(packed)*2, len(packed)+1))
	readCount, readErr := stage.Read(buffer)
	config.Release()

	if readErr != nil && readErr != io.ErrShortBuffer && readErr != io.EOF {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"windows bridge: read failed",
			readErr,
		))
	}

	out := datura.Acquire("windows-bridge-out", datura.APPJSON)

	if _, outErr := out.Write(buffer[:readCount]); outErr != nil {
		out.Release()
		stage.Close()

		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"windows bridge: output decode failed",
			outErr,
		))
	}

	shortWindow = int(datura.Peek[float64](out, "output", "shortWindow"))
	longWindow = int(datura.Peek[float64](out, "output", "longWindow"))
	out.Release()
	stage.Close()

	return shortWindow, longWindow, nil
}
