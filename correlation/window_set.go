package correlation

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
WindowSet retains one member interval history for tiered contagion inputs.
*/
type WindowSet struct {
	artifact *datura.Artifact
}

type windowBranch struct {
	lastLevel float64
	lastEpoch float64
	starts    []float64
	ends      []float64
	rets      []float64
	root      []any
}

/*
NewWindowSet creates a bounded interval accumulator wired from config attributes on the artifact.
*/
func NewWindowSet(artifact *datura.Artifact) *WindowSet {
	return &WindowSet{
		artifact: artifact,
	}
}

func (windowSet *WindowSet) Read(p []byte) (int, error) {
	if windowSet == nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute window set",
			WindowSetError(WindowSetErrorNilReceiver),
		))
	}

	state := datura.Acquire("window-set-state", datura.APPJSON)

	if _, err := state.Write(windowSet.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"correlation-window-set: state write failed",
			err,
		))
	}

	epoch, level, err := wireEpochLevel(windowSet.artifact, state, "window-set")

	if err != nil {
		return 0, err
	}

	if level <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute window set",
			WindowSetError(WindowSetErrorRequirePositiveLevel),
		))
	}

	windowSet.ingest(windowSet.capacityFromArtifact(), epoch, level)

	magnitude := windowSet.lastReturnMagnitude()

	if magnitude <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute window set",
			WindowSetError(WindowSetErrorInsufficientIntervals),
		))
	}

	state.MergeOutput("value", magnitude)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
	return state.Read(p)
}

func (windowSet *WindowSet) Write(p []byte) (int, error) {
	reset := inboundReset(p)
	preserved := windowSet.preserveBranch()

	windowSet.artifact.WithPayload(p)

	if reset {
		return len(p), nil
	}

	windowSet.restoreBranch(preserved)
	return len(p), nil
}

func (windowSet *WindowSet) Close() error {
	return nil
}

func (windowSet *WindowSet) capacityFromArtifact() int {
	capacity := int(datura.Peek[float64](windowSet.artifact, "config", "capacity"))

	if capacity <= 0 {
		capacity = 16
	}

	return capacity
}

func (windowSet *WindowSet) branchPath(root ...any) []any {
	path := make([]any, 0, len(root)+1)
	path = append(path, "interval")
	path = append(path, root...)

	return path
}

func (windowSet *WindowSet) preserveBranch(root ...any) windowBranch {
	base := windowSet.branchPath(root...)

	return windowBranch{
		lastLevel: datura.Peek[float64](windowSet.artifact, append(base, "lastLevel")...),
		lastEpoch: datura.Peek[float64](windowSet.artifact, append(base, "lastEpoch")...),
		starts:    datura.Peek[[]float64](windowSet.artifact, append(base, "starts")...),
		ends:      datura.Peek[[]float64](windowSet.artifact, append(base, "ends")...),
		rets:      datura.Peek[[]float64](windowSet.artifact, append(base, "rets")...),
		root:      append([]any(nil), root...),
	}
}

func (windowSet *WindowSet) restoreBranch(preserved windowBranch) {
	if preserved.lastLevel <= 0 && preserved.lastEpoch <= 0 && len(preserved.rets) == 0 {
		return
	}

	base := windowSet.branchPath(preserved.root...)
	windowSet.artifact.Poke(preserved.lastLevel, append(base, "lastLevel")...)
	windowSet.artifact.Poke(preserved.lastEpoch, append(base, "lastEpoch")...)
	windowSet.artifact.Poke(preserved.starts, append(base, "starts")...)
	windowSet.artifact.Poke(preserved.ends, append(base, "ends")...)
	windowSet.artifact.Poke(preserved.rets, append(base, "rets")...)
}

func (windowSet *WindowSet) ingest(capacity int, epoch int64, level float64, root ...any) {
	if capacity <= 0 {
		capacity = 16
	}

	if level <= 0 {
		return
	}

	base := windowSet.branchPath(root...)
	lastLevel := datura.Peek[float64](windowSet.artifact, append(base, "lastLevel")...)
	lastEpoch := int64(datura.Peek[float64](windowSet.artifact, append(base, "lastEpoch")...))

	if lastLevel <= 0 || lastEpoch <= 0 {
		windowSet.artifact.Poke(level, append(base, "lastLevel")...)
		windowSet.artifact.Poke(float64(epoch), append(base, "lastEpoch")...)

		return
	}

	if epoch <= lastEpoch {
		windowSet.artifact.Poke(level, append(base, "lastLevel")...)

		return
	}

	starts := datura.Peek[[]float64](windowSet.artifact, append(base, "starts")...)
	ends := datura.Peek[[]float64](windowSet.artifact, append(base, "ends")...)
	rets := datura.Peek[[]float64](windowSet.artifact, append(base, "rets")...)

	starts = append(starts, float64(lastEpoch))
	ends = append(ends, float64(epoch))
	rets = append(rets, math.Log(level/lastLevel))

	if len(starts) > capacity {
		trim := len(starts) - capacity
		starts = starts[trim:]
		ends = ends[trim:]
		rets = rets[trim:]
	}

	windowSet.artifact.Poke(starts, append(base, "starts")...)
	windowSet.artifact.Poke(ends, append(base, "ends")...)
	windowSet.artifact.Poke(rets, append(base, "rets")...)
	windowSet.artifact.Poke(level, append(base, "lastLevel")...)
	windowSet.artifact.Poke(float64(epoch), append(base, "lastEpoch")...)
}

func (windowSet *WindowSet) lastReturnMagnitude(root ...any) float64 {
	rets := datura.Peek[[]float64](windowSet.artifact, append(windowSet.branchPath(root...), "rets")...)

	if len(rets) == 0 {
		return 0
	}

	return math.Abs(rets[len(rets)-1])
}

type WindowSetErrorType string

const (
	WindowSetErrorNilReceiver           WindowSetErrorType = "require non-nil window set stage"
	WindowSetErrorRequirePositiveLevel  WindowSetErrorType = "require positive paired level"
	WindowSetErrorInsufficientIntervals WindowSetErrorType = "require at least one log-return interval"
)

type WindowSetError string

func (windowSetError WindowSetError) Error() string {
	return string(windowSetError)
}
