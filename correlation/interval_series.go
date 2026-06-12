package correlation

import "math"

/*
ReturnInterval is a log return over the half-open time interval (start, end] in Unix
nanoseconds. Hayashi-Yoshida sums products of returns whose intervals overlap.
*/
type ReturnInterval struct {
	Start int64
	End   int64
	Ret   float64
}

/*
IntervalSeries accumulates a bounded, time-ordered series of log-return intervals for
one asset from asynchronous trade prints.
*/
type IntervalSeries struct {
	intervals []ReturnInterval
	capacity  int
	lastPrice float64
	lastNanos int64
}

/*
NewIntervalSeries creates a bounded interval accumulator.
*/
func NewIntervalSeries(capacity int) *IntervalSeries {
	if capacity < 1 {
		capacity = 1
	}

	return &IntervalSeries{
		intervals: make([]ReturnInterval, 0, capacity),
		capacity:  capacity,
	}
}

/*
Observe folds one trade print. The first print seeds the anchor; out-of-order timestamps
advance the price anchor without emitting a zero-width interval.
*/
func (series *IntervalSeries) Observe(nanos int64, price float64) {
	if series == nil || price <= 0 {
		return
	}

	if series.lastPrice <= 0 || series.lastNanos <= 0 {
		series.lastPrice = price
		series.lastNanos = nanos

		return
	}

	if nanos <= series.lastNanos {
		series.lastPrice = price

		return
	}

	series.intervals = append(series.intervals, ReturnInterval{
		Start: series.lastNanos,
		End:   nanos,
		Ret:   math.Log(price / series.lastPrice),
	})

	if len(series.intervals) > series.capacity {
		series.intervals = series.intervals[len(series.intervals)-series.capacity:]
	}

	series.lastPrice = price
	series.lastNanos = nanos
}

func (series *IntervalSeries) Len() int {
	if series == nil {
		return 0
	}

	return len(series.intervals)
}

/*
Trim keeps only the most recent keep intervals.
*/
func (series *IntervalSeries) Trim(keep int) {
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
func (series *IntervalSeries) LastReturnMagnitude() float64 {
	if series == nil || len(series.intervals) == 0 {
		return 0
	}

	last := series.intervals[len(series.intervals)-1]

	return math.Abs(last.Ret)
}

/*
RealizedVolatility is the root mean square of interval log returns.
*/
func (series *IntervalSeries) RealizedVolatility() float64 {
	if series == nil || len(series.intervals) == 0 {
		return 0
	}

	total := series.RealizedVariance()

	return math.Sqrt(total / float64(len(series.intervals)))
}

/*
RealizedVolatilityExcludingLast estimates vol before the most recent interval.
*/
func (series *IntervalSeries) RealizedVolatilityExcludingLast() float64 {
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
func (series *IntervalSeries) Clone() *IntervalSeries {
	if series == nil {
		return nil
	}

	copied := make([]ReturnInterval, len(series.intervals))
	copy(copied, series.intervals)

	return &IntervalSeries{
		intervals: copied,
		capacity:  series.capacity,
		lastPrice: series.lastPrice,
		lastNanos: series.lastNanos,
	}
}

/*
CloneTail returns a snapshot containing at most the last window intervals.
*/
func (series *IntervalSeries) CloneTail(window int) *IntervalSeries {
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
func (series *IntervalSeries) RealizedVariance() float64 {
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
func IntervalCorrelation(left, right *IntervalSeries) (float64, bool) {
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
