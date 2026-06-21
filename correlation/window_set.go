package correlation

import (
	"math"

	"github.com/theapemachine/datura"
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
	artifact.Inspect("correlation", "window-set", "NewWindowSet()")

	return &WindowSet{
		artifact: artifact,
	}
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

func (windowSet *WindowSet) Read(p []byte) (int, error) {
	if windowSet == nil {
		return 0, nil
	}

	state := datura.Acquire("window-set-state", datura.APPJSON)
	state.Inspect("correlation", "window-set", "Read()", "p")

	if _, err := state.Write(windowSet.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	level := datura.Peek[float64](state, "paired")

	if level <= 0 {
		return state.Read(p)
	}

	epoch := int64(datura.Peek[float64](state, "sample"))
	windowSet.ingest(windowSet.capacityFromArtifact(), epoch, level)
	state.MergeOutput("value", windowSet.lastReturnMagnitude())
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(p)
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
