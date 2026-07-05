package correlation

import (
	"math"

	"github.com/theapemachine/nomagique/statistic"
)

type intervalSlices struct {
	starts []float64
	ends   []float64
	rets   []float64
}

func medianPairwiseAbsCorrelation(series []intervalSlices) float64 {
	if len(series) < 2 {
		return 0
	}

	correlations := make([]float64, 0, len(series)*(len(series)-1)/2)

	for left := 0; left < len(series); left++ {
		for right := left + 1; right < len(series); right++ {
			value, ok := intervalCorrelationSlices(
				series[left].starts, series[left].ends, series[left].rets,
				series[right].starts, series[right].ends, series[right].rets,
			)

			if !ok {
				continue
			}

			correlations = append(correlations, math.Abs(value))
		}
	}

	if len(correlations) == 0 {
		return 0
	}

	median, ok := statistic.MedianOf(correlations)

	if !ok {
		return 0
	}

	return median
}
