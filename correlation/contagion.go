package correlation

import (
	"math"

	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/statistic"
	"gonum.org/v1/gonum/stat"
)

/*
TierReadings holds median pairwise coupling estimates at fast, medium, and slow scales.
*/
type TierReadings struct {
	Fast   float64
	Medium float64
	Slow   float64
}

/*
ContagionConfig controls cross-universe coupling estimation.
*/
type ContagionConfig struct {
	MinSamples     int
	SymbolCap      int
	AdaptiveSigma  float64
	SpreadCapacity int
}

/*
Contagion estimates universe coupling from multi-tier Hayashi-Yoshida snapshots and
adapts the published reading from fast-medium-slow spread dynamics.
*/
type Contagion struct {
	config ContagionConfig
	spread spreadRing
}

/*
NewContagion creates a contagion estimator with normalized defaults for zero fields.
*/
func NewContagion(config ContagionConfig) *Contagion {
	if config.MinSamples <= 0 {
		config.MinSamples = 16
	}

	if config.SymbolCap <= 0 {
		config.SymbolCap = 16
	}

	if config.AdaptiveSigma <= 0 {
		config.AdaptiveSigma = 2
	}

	if config.SpreadCapacity <= 0 {
		config.SpreadCapacity = 64
	}

	return &Contagion{
		config: config,
		spread: newSpreadRing(config.SpreadCapacity),
	}
}

/*
Observe ingests one cross-section of window snapshots and returns the adaptive coupling.
*/
func (contagion *Contagion) Observe(snapshots []WindowSnapshot) float64 {
	if contagion == nil {
		return 0
	}

	fastSeries, mediumSeries, slowSeries := CollectTierSeries(
		snapshots,
		contagion.config.MinSamples,
		contagion.config.SymbolCap,
	)

	readings := TierReadingsFromSeries(fastSeries, mediumSeries, slowSeries)

	if readings.Medium <= 0 && readings.Fast <= 0 && readings.Slow <= 0 {
		return 0
	}

	return contagion.adaptive(readings)
}

/*
CollectTierSeries gathers fast, medium, and slow interval series that satisfy minSamples
until each tier reaches symbolCap or snapshots are exhausted.
*/
func CollectTierSeries(
	snapshots []WindowSnapshot,
	minSamples int,
	symbolCap int,
) (fastSeries, mediumSeries, slowSeries []*IntervalSeries) {
	if minSamples <= 0 {
		minSamples = 1
	}

	if symbolCap <= 0 {
		symbolCap = 1
	}

	fastSeries = make([]*IntervalSeries, 0, symbolCap)
	mediumSeries = make([]*IntervalSeries, 0, symbolCap)
	slowSeries = make([]*IntervalSeries, 0, symbolCap)

	for _, snapshot := range snapshots {
		if series := snapshot.Fast; series != nil && series.Len() >= minSamples {
			fastSeries = append(fastSeries, series)
		}

		if series := snapshot.Medium; series != nil && series.Len() >= minSamples {
			mediumSeries = append(mediumSeries, series)
		}

		if series := snapshot.Slow; series != nil && series.Len() >= minSamples {
			slowSeries = append(slowSeries, series)
		}

		minCount := len(fastSeries)

		if len(mediumSeries) < minCount {
			minCount = len(mediumSeries)
		}

		if len(slowSeries) < minCount {
			minCount = len(slowSeries)
		}

		if minCount >= symbolCap {
			break
		}
	}

	return fastSeries, mediumSeries, slowSeries
}

/*
TierReadingsFromSeries computes median absolute pairwise correlation per tier.
*/
func TierReadingsFromSeries(
	fastSeries, mediumSeries, slowSeries []*IntervalSeries,
) TierReadings {
	return TierReadings{
		Fast:   MedianPairwiseAbsCorrelation(fastSeries),
		Medium: MedianPairwiseAbsCorrelation(mediumSeries),
		Slow:   MedianPairwiseAbsCorrelation(slowSeries),
	}
}

/*
MedianPairwiseAbsCorrelation returns the median absolute Hayashi-Yoshida correlation
across all symbol pairs in the slice.
*/
func MedianPairwiseAbsCorrelation(series []*IntervalSeries) float64 {
	if len(series) < 2 {
		return 0
	}

	correlations := make([]float64, 0, len(series)*(len(series)-1)/2)

	for left := 0; left < len(series); left++ {
		for right := left + 1; right < len(series); right++ {
			value, ok := IntervalCorrelation(series[left], series[right])

			if !ok {
				continue
			}

			correlations = append(correlations, math.Abs(value))
		}
	}

	if len(correlations) == 0 {
		return 0
	}

	return float64(statistic.NewQuantile(0.5, stat.LinInterp, nil).Observe(
		nomagique.Numbers(correlations...)...,
	))
}

func (contagion *Contagion) adaptive(readings TierReadings) float64 {
	if readings.Fast <= 0 && readings.Medium <= 0 {
		return readings.Slow
	}

	if readings.Slow <= 0 {
		if readings.Medium > 0 {
			return readings.Medium
		}

		return readings.Fast
	}

	spread := readings.Fast - readings.Slow
	contagion.spread.push(spread)

	threshold := adaptiveSpreadThreshold(
		&contagion.spread,
		readings.Slow,
		contagion.config.AdaptiveSigma,
	)

	if spread > threshold {
		return readings.Fast
	}

	if readings.Medium > 0 {
		return readings.Medium
	}

	return readings.Slow
}

func adaptiveSpreadThreshold(
	spreadHistory *spreadRing,
	slowBaseline float64,
	sigma float64,
) float64 {
	if spreadHistory == nil || spreadHistory.len() < 4 {
		if slowBaseline > 0 {
			return slowBaseline
		}

		return 0
	}

	mean, stddev := spreadHistory.meanStdDev()
	floor := mean * mean / (mean + slowBaseline)

	if stddev <= 0 {
		return math.Max(floor, mean)
	}

	return math.Max(floor, mean+sigma*stddev)
}

type spreadRing struct {
	values []float64
	head   int
	count  int
}

func newSpreadRing(capacity int) spreadRing {
	if capacity <= 0 {
		capacity = 1
	}

	return spreadRing{values: make([]float64, capacity)}
}

func (ring *spreadRing) push(value float64) {
	capacity := len(ring.values)
	ring.values[ring.head] = value
	ring.head = (ring.head + 1) % capacity

	if ring.count < capacity {
		ring.count++
	}
}

func (ring *spreadRing) len() int {
	return ring.count
}

func (ring *spreadRing) at(index int) float64 {
	if index < 0 || index >= ring.count {
		return 0
	}

	start := 0

	if ring.count >= len(ring.values) {
		start = ring.head
	}

	return ring.values[(start+index)%len(ring.values)]
}

func (ring *spreadRing) meanStdDev() (mean float64, stddev float64) {
	if ring.count == 0 {
		return 0, 0
	}

	sum := 0.0

	for index := 0; index < ring.count; index++ {
		sum += ring.at(index)
	}

	mean = sum / float64(ring.count)

	if ring.count < 2 {
		return mean, 0
	}

	variance := 0.0

	for index := 0; index < ring.count; index++ {
		delta := ring.at(index) - mean
		variance += delta * delta
	}

	return mean, math.Sqrt(variance / float64(ring.count-1))
}
