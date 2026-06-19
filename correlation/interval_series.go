package correlation

import (
	"math"

	"github.com/theapemachine/datura"
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
NewIntervalSeries creates a bounded interval accumulator.
*/
func NewIntervalSeries() *IntervalSeries {
	return &IntervalSeries{
		artifact: datura.Acquire("interval-series", datura.APPJSON),
	}
}

func (series *IntervalSeries) Write(p []byte) (int, error) {
	reset := inboundReset(p)
	preserved := series.preserveBranch()

	n, err := series.artifact.Write(p)

	if reset {
		return n, err
	}

	series.restoreBranch(preserved)

	return n, err
}

func (series *IntervalSeries) Read(p []byte) (int, error) {
	level := datura.Peek[float64](series.artifact, "paired")

	if level <= 0 {
		return series.artifact.Read(p)
	}

	epoch := int64(datura.Peek[float64](series.artifact, "sample"))
	series.ingest(series.capacityFromArtifact(), epoch, level)

	series.artifact.Poke(datura.Map[float64]{
		"value": series.lastReturnMagnitude(),
	}, "output")

	return series.artifact.Read(p)
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
