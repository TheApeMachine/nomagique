package correlation

import (
	"math"

	"github.com/theapemachine/datura"
)

/*
IntervalCoupling tracks Hayashi-Yoshida correlation between left and right interval streams.
Write with config.side 0 for the left stream and 1 for the right stream.
*/
type IntervalCoupling struct {
	artifact *datura.Artifact
}

type couplingBranch struct {
	lastLevel float64
	lastEpoch float64
	starts    []float64
	ends      []float64
	rets      []float64
	side      string
}

/*
NewIntervalCoupling creates an interval-correlation stage over left and right streams.
*/
func NewIntervalCoupling() *IntervalCoupling {
	return &IntervalCoupling{
		artifact: datura.Acquire("interval-coupling", datura.APPJSON),
	}
}

func (coupling *IntervalCoupling) Write(p []byte) (int, error) {
	leftPreserved := coupling.preserveBranch("left")
	rightPreserved := coupling.preserveBranch("right")

	n, err := coupling.artifact.Write(p)

	coupling.restoreBranch(leftPreserved)
	coupling.restoreBranch(rightPreserved)

	return n, err
}

func (coupling *IntervalCoupling) Read(p []byte) (int, error) {
	if coupling == nil {
		return 0, nil
	}

	level := datura.Peek[float64](coupling.artifact, "paired")

	if level > 0 {
		epoch := int64(datura.Peek[float64](coupling.artifact, "sample"))
		side := "left"

		if int(datura.Peek[float64](coupling.artifact, "config", "side")) == 1 {
			side = "right"
		}

		coupling.ingest(coupling.capacityFromArtifact(), epoch, level, side)
	}

	value, ok := coupling.correlateSides()

	if !ok {
		coupling.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return coupling.artifact.Read(p)
	}

	coupling.artifact.Poke(datura.Map[float64]{"value": value}, "output")

	return coupling.artifact.Read(p)
}

func (coupling *IntervalCoupling) Close() error {
	return nil
}

func (coupling *IntervalCoupling) capacityFromArtifact() int {
	capacity := int(datura.Peek[float64](coupling.artifact, "config", "capacity"))

	if capacity <= 0 {
		capacity = 8
	}

	return capacity
}

func (coupling *IntervalCoupling) branchPath(side string) []any {
	return []any{"interval", side}
}

func (coupling *IntervalCoupling) preserveBranch(side string) couplingBranch {
	base := coupling.branchPath(side)

	return couplingBranch{
		lastLevel: datura.Peek[float64](coupling.artifact, append(base, "lastLevel")...),
		lastEpoch: datura.Peek[float64](coupling.artifact, append(base, "lastEpoch")...),
		starts:    datura.Peek[[]float64](coupling.artifact, append(base, "starts")...),
		ends:      datura.Peek[[]float64](coupling.artifact, append(base, "ends")...),
		rets:      datura.Peek[[]float64](coupling.artifact, append(base, "rets")...),
		side:      side,
	}
}

func (coupling *IntervalCoupling) restoreBranch(preserved couplingBranch) {
	if preserved.lastLevel <= 0 && preserved.lastEpoch <= 0 && len(preserved.rets) == 0 {
		return
	}

	base := coupling.branchPath(preserved.side)
	coupling.artifact.Poke(preserved.lastLevel, append(base, "lastLevel")...)
	coupling.artifact.Poke(preserved.lastEpoch, append(base, "lastEpoch")...)
	coupling.artifact.Poke(preserved.starts, append(base, "starts")...)
	coupling.artifact.Poke(preserved.ends, append(base, "ends")...)
	coupling.artifact.Poke(preserved.rets, append(base, "rets")...)
}

func (coupling *IntervalCoupling) ingest(capacity int, epoch int64, level float64, side string) {
	if capacity <= 0 {
		capacity = 8
	}

	if level <= 0 {
		return
	}

	base := coupling.branchPath(side)
	lastLevel := datura.Peek[float64](coupling.artifact, append(base, "lastLevel")...)
	lastEpoch := int64(datura.Peek[float64](coupling.artifact, append(base, "lastEpoch")...))

	if lastLevel <= 0 || lastEpoch <= 0 {
		coupling.artifact.Poke(level, append(base, "lastLevel")...)
		coupling.artifact.Poke(float64(epoch), append(base, "lastEpoch")...)

		return
	}

	if epoch <= lastEpoch {
		coupling.artifact.Poke(level, append(base, "lastLevel")...)

		return
	}

	starts := datura.Peek[[]float64](coupling.artifact, append(base, "starts")...)
	ends := datura.Peek[[]float64](coupling.artifact, append(base, "ends")...)
	rets := datura.Peek[[]float64](coupling.artifact, append(base, "rets")...)

	starts = append(starts, float64(lastEpoch))
	ends = append(ends, float64(epoch))
	rets = append(rets, math.Log(level/lastLevel))

	if len(starts) > capacity {
		trim := len(starts) - capacity
		starts = starts[trim:]
		ends = ends[trim:]
		rets = rets[trim:]
	}

	coupling.artifact.Poke(starts, append(base, "starts")...)
	coupling.artifact.Poke(ends, append(base, "ends")...)
	coupling.artifact.Poke(rets, append(base, "rets")...)
	coupling.artifact.Poke(level, append(base, "lastLevel")...)
	coupling.artifact.Poke(float64(epoch), append(base, "lastEpoch")...)
}

func (coupling *IntervalCoupling) correlateSides() (float64, bool) {
	leftStarts, leftEnds, leftRets := coupling.peekBranch("left")
	rightStarts, rightEnds, rightRets := coupling.peekBranch("right")

	return intervalCorrelationSlices(
		leftStarts, leftEnds, leftRets,
		rightStarts, rightEnds, rightRets,
	)
}

func (coupling *IntervalCoupling) peekBranch(side string) (starts, ends, rets []float64) {
	base := coupling.branchPath(side)
	starts = append([]float64(nil), datura.Peek[[]float64](coupling.artifact, append(base, "starts")...)...)
	ends = append([]float64(nil), datura.Peek[[]float64](coupling.artifact, append(base, "ends")...)...)
	rets = append([]float64(nil), datura.Peek[[]float64](coupling.artifact, append(base, "rets")...)...)

	return starts, ends, rets
}

func intervalCorrelationSlices(
	leftStarts, leftEnds, leftRets,
	rightStarts, rightEnds, rightRets []float64,
) (float64, bool) {
	varLeft := realizedVariance(leftRets)
	varRight := realizedVariance(rightRets)

	if varLeft <= 0 || varRight <= 0 {
		return 0, false
	}

	covariance := intervalCovariance(leftStarts, leftEnds, leftRets, rightStarts, rightEnds, rightRets)
	correlation := covariance / math.Sqrt(varLeft*varRight)

	if correlation > 1 {
		return 1, true
	}

	if correlation < -1 {
		return -1, true
	}

	return correlation, true
}

func realizedVariance(rets []float64) float64 {
	total := 0.0

	for _, ret := range rets {
		total += ret * ret
	}

	return total
}

func intervalCovariance(
	leftStarts, leftEnds, leftRets,
	rightStarts, rightEnds, rightRets []float64,
) float64 {
	covariance := 0.0
	window := 0

	for leftIndex := range leftStarts {
		leftStart := int64(leftStarts[leftIndex])
		leftEnd := int64(leftEnds[leftIndex])
		leftRet := leftRets[leftIndex]

		for window < len(rightStarts) && int64(rightEnds[window]) <= leftStart {
			window++
		}

		for index := window; index < len(rightStarts) && int64(rightStarts[index]) < leftEnd; index++ {
			covariance += leftRet * rightRets[index]
		}
	}

	return covariance
}
