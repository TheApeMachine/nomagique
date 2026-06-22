package correlation

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
IntervalSeries accumulates a bounded, time-ordered series of log-return intervals from
paired epoch and level observations. Retained state lives under interval.* payload keys.
*/
type IntervalSeries struct {
	artifact *datura.Artifact
}

type intervalBranch struct {
	lastLevel float64
	lastEpoch float64
	starts    []float64
	ends      []float64
	rets      []float64
	root      []any
}

/*
NewIntervalSeries creates a bounded interval accumulator wired from config attributes on the artifact.
*/
func NewIntervalSeries(artifact *datura.Artifact) *IntervalSeries {
	artifact.Inspect("correlation", "interval-series", "NewIntervalSeries()")

	return &IntervalSeries{
		artifact: artifact,
	}
}

func (series *IntervalSeries) Write(p []byte) (int, error) {
	reset := inboundReset(p)
	preserved := series.preserveBranch()

	series.artifact.WithPayload(p)

	if reset {
		return len(p), nil
	}

	series.restoreBranch(preserved)
	return len(p), nil
}

func (series *IntervalSeries) Read(p []byte) (int, error) {
	state := datura.Acquire("interval-series-state", datura.APPJSON)
	state.Inspect("correlation", "interval-series", "Read()", "p")

	if _, err := state.Write(series.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	level := datura.Peek[float64](state, "paired")

	if level <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute interval series",
			IntervalSeriesError(IntervalSeriesErrorRequirePositiveLevel),
		))
	}

	epoch := int64(datura.Peek[float64](state, "sample"))
	series.ingest(series.capacityFromArtifact(), epoch, level)

	magnitude := series.lastReturnMagnitude()

	if magnitude <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute interval series",
			IntervalSeriesError(IntervalSeriesErrorInsufficientIntervals),
		))
	}

	state.MergeOutput("value", magnitude)
	state.Merge("root", "output")
	state.Merge("inputs", []string{"value"})
	return state.Read(p)
}

func (series *IntervalSeries) Close() error {
	return nil
}

func (series *IntervalSeries) capacityFromArtifact() int {
	capacity := int(datura.Peek[float64](series.artifact, "config", "capacity"))

	if capacity <= 0 {
		capacity = 8
	}

	return capacity
}

func (series *IntervalSeries) branchPath(root ...any) []any {
	path := make([]any, 0, len(root)+1)
	path = append(path, "interval")
	path = append(path, root...)

	return path
}

func (series *IntervalSeries) preserveBranch(root ...any) intervalBranch {
	base := series.branchPath(root...)

	return intervalBranch{
		lastLevel: datura.Peek[float64](series.artifact, append(base, "lastLevel")...),
		lastEpoch: datura.Peek[float64](series.artifact, append(base, "lastEpoch")...),
		starts:    datura.Peek[[]float64](series.artifact, append(base, "starts")...),
		ends:      datura.Peek[[]float64](series.artifact, append(base, "ends")...),
		rets:      datura.Peek[[]float64](series.artifact, append(base, "rets")...),
		root:      append([]any(nil), root...),
	}
}

func (series *IntervalSeries) restoreBranch(preserved intervalBranch) {
	if preserved.lastLevel <= 0 && preserved.lastEpoch <= 0 && len(preserved.rets) == 0 {
		return
	}

	base := series.branchPath(preserved.root...)
	series.artifact.Poke(preserved.lastLevel, append(base, "lastLevel")...)
	series.artifact.Poke(preserved.lastEpoch, append(base, "lastEpoch")...)
	series.artifact.Poke(preserved.starts, append(base, "starts")...)
	series.artifact.Poke(preserved.ends, append(base, "ends")...)
	series.artifact.Poke(preserved.rets, append(base, "rets")...)
}

func (series *IntervalSeries) ingest(capacity int, epoch int64, level float64, root ...any) {
	if capacity <= 0 {
		capacity = 8
	}

	if level <= 0 {
		return
	}

	base := series.branchPath(root...)
	lastLevel := datura.Peek[float64](series.artifact, append(base, "lastLevel")...)
	lastEpoch := int64(datura.Peek[float64](series.artifact, append(base, "lastEpoch")...))

	if lastLevel <= 0 || lastEpoch <= 0 {
		series.artifact.Poke(level, append(base, "lastLevel")...)
		series.artifact.Poke(float64(epoch), append(base, "lastEpoch")...)

		return
	}

	if epoch <= lastEpoch {
		series.artifact.Poke(level, append(base, "lastLevel")...)

		return
	}

	starts := datura.Peek[[]float64](series.artifact, append(base, "starts")...)
	ends := datura.Peek[[]float64](series.artifact, append(base, "ends")...)
	rets := datura.Peek[[]float64](series.artifact, append(base, "rets")...)

	starts = append(starts, float64(lastEpoch))
	ends = append(ends, float64(epoch))
	rets = append(rets, math.Log(level/lastLevel))

	if len(starts) > capacity {
		trim := len(starts) - capacity
		starts = starts[trim:]
		ends = ends[trim:]
		rets = rets[trim:]
	}

	series.artifact.Poke(starts, append(base, "starts")...)
	series.artifact.Poke(ends, append(base, "ends")...)
	series.artifact.Poke(rets, append(base, "rets")...)
	series.artifact.Poke(level, append(base, "lastLevel")...)
	series.artifact.Poke(float64(epoch), append(base, "lastEpoch")...)
}

func (series *IntervalSeries) lastReturnMagnitude(root ...any) float64 {
	rets := datura.Peek[[]float64](series.artifact, append(series.branchPath(root...), "rets")...)

	if len(rets) == 0 {
		return 0
	}

	return math.Abs(rets[len(rets)-1])
}

type IntervalSeriesErrorType string

const (
	IntervalSeriesErrorRequirePositiveLevel IntervalSeriesErrorType = "require positive paired level"
	IntervalSeriesErrorInsufficientIntervals IntervalSeriesErrorType = "require at least one log-return interval"
)

type IntervalSeriesError string

func (intervalSeriesError IntervalSeriesError) Error() string {
	return string(intervalSeriesError)
}
