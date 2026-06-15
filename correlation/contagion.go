package correlation

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/statistic"
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
ContagionConfig controls cross-member coupling estimation.
*/
type ContagionConfig struct {
	MinSamples     int
	MemberCap      int
	AdaptiveSigma  float64
	SpreadCapacity int
}

/*
Contagion estimates ensemble coupling from multi-tier interval snapshots and adapts
the published reading from fast-medium-slow spread dynamics.
*/
type Contagion struct {
	artifact   *datura.Artifact
	windowSets []*WindowSet
	tiers      TierWindows
	config     ContagionConfig
	spread     spreadRing
	output     float64
}

/*
NewContagion wires window sets into a coupling estimator.
*/
func NewContagion(
	windowSets []*WindowSet,
	tiers TierWindows,
	config ContagionConfig,
) *Contagion {
	if config.MinSamples <= 0 {
		config.MinSamples = 16
	}

	if config.MemberCap <= 0 {
		config.MemberCap = 16
	}

	if config.AdaptiveSigma <= 0 {
		config.AdaptiveSigma = 2
	}

	if config.SpreadCapacity <= 0 {
		config.SpreadCapacity = 64
	}

	return &Contagion{
		artifact:   datura.Acquire("contagion", datura.Artifact_Type_json),
		windowSets: windowSets,
		tiers:      tiers,
		config:     config,
		spread:     newSpreadRing(config.SpreadCapacity),
	}
}

func (contagion *Contagion) Write(p []byte) (int, error) {
	return contagion.artifact.Write(p)
}

func (contagion *Contagion) Read(p []byte) (int, error) {
	if contagion == nil {
		return contagion.artifact.Read(p)
	}

	snapshots := contagion.snapshots()

	if len(snapshots) > 0 {
		contagion.output = contagion.observeSnapshots(snapshots)
		putFloat64Payload(&contagion.artifact, "contagion", contagion.output)
	}

	return contagion.artifact.Read(p)
}

func (contagion *Contagion) Close() error {
	return nil
}

/*
Reset clears spread history.
*/
func (contagion *Contagion) Reset() error {
	if contagion == nil {
		return nil
	}

	contagion.spread = newSpreadRing(contagion.config.SpreadCapacity)
	contagion.output = 0

	return nil
}

/*
TierReadings returns the latest median pairwise readings before adaptive selection.
*/
func (contagion *Contagion) TierReadings() TierReadings {
	snapshots := contagion.snapshots()

	if len(snapshots) == 0 {
		return TierReadings{}
	}

	fastSeries, mediumSeries, slowSeries := CollectTierSeries(
		snapshots,
		contagion.config.MinSamples,
		contagion.config.MemberCap,
	)

	return TierReadingsFromSeries(fastSeries, mediumSeries, slowSeries)
}

func (contagion *Contagion) snapshots() []WindowSnapshot {
	if contagion == nil {
		return nil
	}

	snapshots := make([]WindowSnapshot, 0, len(contagion.windowSets))

	for _, windowSet := range contagion.windowSets {
		if windowSet == nil {
			continue
		}

		snapshot := windowSet.Snapshot(contagion.tiers)

		if snapshot.Fast == nil && snapshot.Medium == nil && snapshot.Slow == nil {
			continue
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots
}

func (contagion *Contagion) observeSnapshots(snapshots []WindowSnapshot) float64 {
	fastSeries, mediumSeries, slowSeries := CollectTierSeries(
		snapshots,
		contagion.config.MinSamples,
		contagion.config.MemberCap,
	)

	readings := TierReadingsFromSeries(fastSeries, mediumSeries, slowSeries)

	if readings.Medium <= 0 && readings.Fast <= 0 && readings.Slow <= 0 {
		return 0
	}

	return contagion.adaptive(readings)
}

/*
CollectTierSeries gathers fast, medium, and slow interval series that satisfy minSamples
until each tier reaches memberCap or snapshots are exhausted.
*/
func CollectTierSeries(
	snapshots []WindowSnapshot,
	minSamples int,
	memberCap int,
) (fastSeries, mediumSeries, slowSeries []*IntervalSeries) {
	if minSamples <= 0 {
		minSamples = 1
	}

	if memberCap <= 0 {
		memberCap = 1
	}

	fastSeries = make([]*IntervalSeries, 0, memberCap)
	mediumSeries = make([]*IntervalSeries, 0, memberCap)
	slowSeries = make([]*IntervalSeries, 0, memberCap)

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

		if minCount >= memberCap {
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
across all series pairs in the slice.
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

	return statistic.MedianOf(correlations)
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
