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

type memberSnapshot struct {
	fast   intervalSlices
	medium intervalSlices
	slow   intervalSlices
}

func (contagion *Contagion) snapshots() []memberSnapshot {
	if contagion == nil {
		return nil
	}

	tiers := contagion.tiersFromArtifact()
	memberIDs := contagion.memberIDsFromConfig()
	snapshots := make([]memberSnapshot, 0, len(memberIDs))

	for _, member := range memberIDs {
		fastStarts, fastEnds, fastRets := contagion.tailBranch(tiers.fast, contagion.memberSegment(member))
		mediumStarts, mediumEnds, mediumRets := contagion.tailBranch(tiers.medium, contagion.memberSegment(member))
		slowStarts, slowEnds, slowRets := contagion.tailBranch(tiers.slow, contagion.memberSegment(member))

		snapshot := memberSnapshot{
			fast: intervalSlices{
				starts: fastStarts,
				ends:   fastEnds,
				rets:   fastRets,
			},
			medium: intervalSlices{
				starts: mediumStarts,
				ends:   mediumEnds,
				rets:   mediumRets,
			},
			slow: intervalSlices{
				starts: slowStarts,
				ends:   slowEnds,
				rets:   slowRets,
			},
		}

		if len(snapshot.fast.rets) == 0 &&
			len(snapshot.medium.rets) == 0 &&
			len(snapshot.slow.rets) == 0 {
			continue
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots
}

func (contagion *Contagion) observeSnapshots(snapshots []memberSnapshot) (float64, tierReadings, error) {
	fastSeries, mediumSeries, slowSeries := collectTierSeries(
		snapshots,
		contagion.minSamplesFromArtifact(),
		contagion.memberCapFromArtifact(),
	)

	readings := tierReadingsFromSeries(fastSeries, mediumSeries, slowSeries)

	if readings.medium <= 0 && readings.fast <= 0 && readings.slow <= 0 {
		return 0, readings, nil
	}

	value, err := contagion.adaptive(readings)

	return value, readings, err
}

func collectTierSeries(
	snapshots []memberSnapshot,
	minSamples int,
	memberCap int,
) (fastSeries, mediumSeries, slowSeries []intervalSlices) {
	if minSamples <= 0 || memberCap <= 0 {
		return nil, nil, nil
	}

	fastSeries = make([]intervalSlices, 0, memberCap)
	mediumSeries = make([]intervalSlices, 0, memberCap)
	slowSeries = make([]intervalSlices, 0, memberCap)

	for _, snapshot := range snapshots {
		if len(snapshot.fast.rets) >= minSamples {
			fastSeries = append(fastSeries, snapshot.fast)
		}

		if len(snapshot.medium.rets) >= minSamples {
			mediumSeries = append(mediumSeries, snapshot.medium)
		}

		if len(snapshot.slow.rets) >= minSamples {
			slowSeries = append(slowSeries, snapshot.slow)
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

type tierReadings struct {
	fast   float64
	medium float64
	slow   float64
}

func tierReadingsFromSeries(
	fastSeries, mediumSeries, slowSeries []intervalSlices,
) tierReadings {
	return tierReadings{
		fast:   medianPairwiseAbsCorrelation(fastSeries),
		medium: medianPairwiseAbsCorrelation(mediumSeries),
		slow:   medianPairwiseAbsCorrelation(slowSeries),
	}
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

func (contagion *Contagion) adaptive(readings tierReadings) (float64, error) {
	if readings.fast <= 0 && readings.medium <= 0 {
		return readings.slow, nil
	}

	if readings.slow <= 0 {
		if readings.medium > 0 {
			return readings.medium, nil
		}

		return readings.fast, nil
	}

	spread := readings.fast - readings.slow
	pushSpread(contagion.artifact, contagion.spreadCapacityFromArtifact(), spread)

	sigma, err := contagion.adaptiveSigmaFromArtifact()

	if err != nil {
		return 0, err
	}

	threshold := adaptiveSpreadThreshold(
		contagion.artifact,
		readings.slow,
		sigma,
	)

	if spread > threshold {
		return readings.fast, nil
	}

	if readings.medium > 0 {
		return readings.medium, nil
	}

	return readings.slow, nil
}
