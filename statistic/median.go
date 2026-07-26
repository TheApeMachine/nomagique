package statistic

import (
	"math"
	"sync"

	"github.com/theapemachine/errnie"
)

var scratchPool = sync.Pool{
	New: func() any {
		s := make([]float64, 0, 1024)
		return &s
	},
}

/*
Median computes the sample median over retained history or panel peers.
*/
type Median struct {
	history []float64
}

/*
NewMedian returns a typed median accumulator.
*/
func NewMedian() *Median {
	return &Median{}
}

/*
Measure adds one sample and returns the median of retained history.
*/
func (median *Median) Measure(sample float64) (ScalarOutput, error) {
	if err := finiteStatistic("median", sample); err != nil {
		return ScalarOutput{}, err
	}

	median.history = append(median.history, sample)
	value, ok := MedianOf(median.history)

	if !ok {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: unable to compute median",
			nil,
		))
	}

	return ScalarOutput{
		Value: value,
		Ready: true,
		Count: len(median.history),
	}, nil
}

/*
MeasurePeers returns the median of peers excluding the requested member.
*/
func (median *Median) MeasurePeers(member string, peers map[string]float64) (ScalarOutput, error) {
	if member == "" {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: member required",
			nil,
		))
	}

	if len(peers) == 0 {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: peers required",
			nil,
		))
	}

	ptr := scratchPool.Get().(*[]float64)
	values := (*ptr)[:0]

	if cap(values) < len(peers) {
		values = make([]float64, 0, len(peers))
	}

	defer func() {
		*ptr = values[:0]
		scratchPool.Put(ptr)
	}()

	for peer, value := range peers {
		if err := finiteStatistic("median", value); err != nil {
			return ScalarOutput{}, err
		}

		if peer == member {
			continue
		}

		values = append(values, value)
	}

	if len(values) == 0 {
		value, ok := peers[member]

		if ok {
			return ScalarOutput{
				Value: value,
				Ready: true,
				Count: len(peers),
			}, nil
		}

		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: no peer samples for member",
			nil,
		))
	}

	value, ok := MedianInPlace(values)

	if !ok {
		return ScalarOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"median: unable to compute peer median",
			nil,
		))
	}

	return ScalarOutput{
		Value: value,
		Ready: true,
		Count: len(peers),
	}, nil
}

/*
MedianOf returns the median of values without weights.
*/
func MedianOf(values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
	}

	ptr := scratchPool.Get().(*[]float64)
	sorted := (*ptr)[:0]
	if cap(sorted) < len(values) {
		sorted = make([]float64, 0, len(values))
	}
	sorted = append(sorted, values...)

	result, ok := MedianInPlace(sorted)
	*ptr = sorted[:0]
	scratchPool.Put(ptr)

	return result, ok
}

/*
MedianInPlace returns the median by partitioning values directly.
It avoids the full sorting allocation/cost for callers that already own scratch.
*/
func MedianInPlace(values []float64) (float64, bool) {
	if len(values) == 0 {
		return 0, false
	}

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, false
		}
	}

	middle := len(values) / 2
	right := medianSelect(values, middle)

	if len(values)%2 == 1 {
		return right, true
	}

	left := medianSelect(values[:middle], middle-1)

	return (left + right) / 2, true
}

func medianSelect(values []float64, target int) float64 {
	left := 0
	right := len(values) - 1

	for left < right {
		pivot := medianPartition(values, left, right)

		if pivot == target {
			break
		}

		if pivot < target {
			left = pivot + 1
			continue
		}

		right = pivot - 1
	}

	return values[target]
}

func medianPartition(values []float64, left int, right int) int {
	pivot := values[right]
	store := left

	for index := left; index < right; index++ {
		if values[index] < pivot {
			values[store], values[index] = values[index], values[store]
			store++
		}
	}

	values[right], values[store] = values[store], values[right]

	return store
}
