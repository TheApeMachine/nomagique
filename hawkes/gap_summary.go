package hawkes

import (
	"math"
	"slices"

	"github.com/theapemachine/errnie"
	"gonum.org/v1/gonum/stat"
)

/*
gapSummary owns the reusable inter-arrival statistics of an arrival stream.
*/
type gapSummary struct {
	values []float64
	sorted []float64
}

func newGapSummary(capacity int) gapSummary {
	return gapSummary{
		values: make([]float64, 0, capacity),
		sorted: make([]float64, 0, capacity),
	}
}

func (summary *gapSummary) reset(marked []MarkedEvent) {
	summary.values = summary.values[:0]

	for index := 1; index < len(marked); index++ {
		gap := marked[index].At.Sub(marked[index-1].At).Seconds()

		if gap > 0 {
			summary.values = append(summary.values, gap)
		}
	}

	summary.sorted = append(summary.sorted[:0], summary.values...)
	slices.Sort(summary.sorted)
}

func (summary gapSummary) median() (float64, bool) {
	if len(summary.sorted) == 0 || !summary.finite() {
		return 0, false
	}

	middle := len(summary.sorted) / 2

	if len(summary.sorted)%2 == 0 {
		return (summary.sorted[middle-1] + summary.sorted[middle]) / 2, true
	}

	return summary.sorted[middle], true
}

func (summary gapSummary) quartiles() (float64, float64, error) {
	if len(summary.sorted) == 0 {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes grid: quartiles require values",
			nil,
		))
	}

	if !summary.finite() {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"hawkes grid: quartiles sample is non-finite",
			nil,
		))
	}

	lower := stat.Quantile(0.25, stat.LinInterp, summary.sorted, nil)
	upper := stat.Quantile(0.75, stat.LinInterp, summary.sorted, nil)

	return lower, upper, nil
}

func (summary gapSummary) finite() bool {
	for _, value := range summary.sorted {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}

	return true
}
