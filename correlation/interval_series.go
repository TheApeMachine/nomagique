package correlation

import (
	"math"

	"github.com/theapemachine/nomagique/core"
)

/*
ReturnInterval is a log return over the half-open time interval (start, end] encoded
as epoch nanoseconds. Hayashi-Yoshida sums products of returns whose intervals overlap.
*/
type ReturnInterval struct {
	Start int64
	End   int64
	Ret   float64
}

/*
IntervalSeries accumulates a bounded, time-ordered series of log-return intervals from
paired epoch and level observations.
*/
type IntervalSeries[T ~float64] struct {
	intervals []ReturnInterval
	capacity  int
	lastLevel float64
	lastEpoch int64
	output    core.Scalar[T]
}

/*
NewIntervalSeries creates a bounded interval accumulator.
*/
func NewIntervalSeries[T ~float64](capacity int) *IntervalSeries[T] {
	if capacity < 1 {
		capacity = 1
	}

	return &IntervalSeries[T]{
		intervals: make([]ReturnInterval, 0, capacity),
		capacity:  capacity,
	}
}

/*
Observe ingests epoch nanoseconds and a positive level as two scalar inputs.
*/
func (series *IntervalSeries[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if series == nil {
		return core.Scalar[T](0)
	}

	epoch, level, ok := parseEpochLevel(inputs...)

	if !ok {
		return series.output
	}

	series.ingest(epoch, level)
	series.output = core.Scalar[T](T(series.LastReturnMagnitude()))

	return series.output
}

/*
Reset clears interval history and anchors.
*/
func (series *IntervalSeries[T]) Reset() error {
	if series == nil {
		return nil
	}

	series.intervals = series.intervals[:0]
	series.lastLevel = 0
	series.lastEpoch = 0
	series.output = core.Scalar[T](0)

	return nil
}

func (series *IntervalSeries[T]) ingest(epoch int64, level float64) {
	if level <= 0 {
		return
	}

	if series.lastLevel <= 0 || series.lastEpoch <= 0 {
		series.lastLevel = level
		series.lastEpoch = epoch

		return
	}

	if epoch <= series.lastEpoch {
		series.lastLevel = level

		return
	}

	series.intervals = append(series.intervals, ReturnInterval{
		Start: series.lastEpoch,
		End:   epoch,
		Ret:   math.Log(level / series.lastLevel),
	})

	if len(series.intervals) > series.capacity {
		series.intervals = series.intervals[len(series.intervals)-series.capacity:]
	}

	series.lastLevel = level
	series.lastEpoch = epoch
}

func (series *IntervalSeries[T]) Len() int {
	if series == nil {
		return 0
	}

	return len(series.intervals)
}

/*
Trim keeps only the most recent keep intervals.
*/
func (series *IntervalSeries[T]) Trim(keep int) {
	if series == nil {
		return
	}

	if keep <= 0 {
		series.intervals = series.intervals[:0]

		return
	}

	if len(series.intervals) <= keep {
		return
	}

	series.intervals = series.intervals[len(series.intervals)-keep:]
}

/*
LastReturnMagnitude is the absolute log return of the most recent interval.
*/
func (series *IntervalSeries[T]) LastReturnMagnitude() float64 {
	if series == nil || len(series.intervals) == 0 {
		return 0
	}

	last := series.intervals[len(series.intervals)-1]

	return math.Abs(last.Ret)
}

/*
RealizedVolatility is the root mean square of interval log returns.
*/
func (series *IntervalSeries[T]) RealizedVolatility() float64 {
	if series == nil || len(series.intervals) == 0 {
		return 0
	}

	total := series.RealizedVariance()

	return math.Sqrt(total / float64(len(series.intervals)))
}

/*
RealizedVolatilityExcludingLast estimates vol before the most recent interval.
*/
func (series *IntervalSeries[T]) RealizedVolatilityExcludingLast() float64 {
	if series == nil || len(series.intervals) <= 1 {
		return series.RealizedVolatility()
	}

	total := 0.0
	count := len(series.intervals) - 1

	for index := 0; index < count; index++ {
		ret := series.intervals[index].Ret
		total += ret * ret
	}

	return math.Sqrt(total / float64(count))
}

/*
Clone returns an independent snapshot of the interval history.
*/
func (series *IntervalSeries[T]) Clone() *IntervalSeries[T] {
	if series == nil {
		return nil
	}

	copied := make([]ReturnInterval, len(series.intervals))
	copy(copied, series.intervals)

	return &IntervalSeries[T]{
		intervals: copied,
		capacity:  series.capacity,
		lastLevel: series.lastLevel,
		lastEpoch: series.lastEpoch,
		output:    series.output,
	}
}

/*
CloneTail returns a snapshot containing at most the last window intervals.
*/
func (series *IntervalSeries[T]) CloneTail(window int) *IntervalSeries[T] {
	cloned := series.Clone()

	if cloned == nil {
		return nil
	}

	if window <= 0 {
		cloned.intervals = cloned.intervals[:0]

		return cloned
	}

	if len(cloned.intervals) > window {
		cloned.intervals = cloned.intervals[len(cloned.intervals)-window:]
	}

	return cloned
}

/*
RealizedVariance is the Hayashi-Yoshida variance of the series against itself.
*/
func (series *IntervalSeries[T]) RealizedVariance() float64 {
	if series == nil {
		return 0
	}

	total := 0.0

	for _, interval := range series.intervals {
		total += interval.Ret * interval.Ret
	}

	return total
}

/*
IntervalCorrelation normalises asynchronous interval covariance by realised standard
deviations. It reports false when either series carries no variance.
*/
func IntervalCorrelation[T ~float64](
	left, right *IntervalSeries[T],
) (float64, bool) {
	if left == nil || right == nil {
		return 0, false
	}

	varLeft := left.RealizedVariance()
	varRight := right.RealizedVariance()

	if varLeft <= 0 || varRight <= 0 {
		return 0, false
	}

	covariance := intervalCovariance(left.intervals, right.intervals)
	correlation := covariance / math.Sqrt(varLeft*varRight)

	if correlation > 1 {
		return 1, true
	}

	if correlation < -1 {
		return -1, true
	}

	return correlation, true
}

func intervalCovariance(left, right []ReturnInterval) float64 {
	covariance := 0.0
	window := 0

	for _, leftInterval := range left {
		for window < len(right) && right[window].End <= leftInterval.Start {
			window++
		}

		for index := window; index < len(right) && right[index].Start < leftInterval.End; index++ {
			covariance += leftInterval.Ret * right[index].Ret
		}
	}

	return covariance
}
