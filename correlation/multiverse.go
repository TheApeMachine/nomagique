package correlation

import (
	"github.com/theapemachine/nomagique/core"
)

/*
Multiverse estimates cross-symbol coupling from tiered window snapshots.
Feed each WindowSet via Observe on the underlying series, then call Multiverse.Observe.
*/
type Multiverse struct {
	windowSets []*WindowSet
	tiers      TierWindows
	contagion  *Contagion
}

/*
NewMultiverse wires window sets into a contagion estimator.
*/
func NewMultiverse(
	windowSets []*WindowSet,
	tiers TierWindows,
	config ContagionConfig,
) *Multiverse {
	return &Multiverse{
		windowSets: windowSets,
		tiers:      tiers,
		contagion:  NewContagion(config),
	}
}

/*
Observe materializes tier snapshots from every window set and returns adaptive coupling.
*/
func (multiverse *Multiverse) Observe(_ ...core.Number) core.Float64 {
	if multiverse == nil || multiverse.contagion == nil {
		return 0
	}

	snapshots := multiverse.snapshots()

	if len(snapshots) == 0 {
		return 0
	}

	return core.Float64(multiverse.contagion.Observe(snapshots))
}

/*
TierReadings returns the latest median pairwise readings before adaptive selection.
*/
func (multiverse *Multiverse) TierReadings() TierReadings {
	snapshots := multiverse.snapshots()

	if len(snapshots) == 0 {
		return TierReadings{}
	}

	fastSeries, mediumSeries, slowSeries := CollectTierSeries(
		snapshots,
		multiverse.contagion.config.MinSamples,
		multiverse.contagion.config.SymbolCap,
	)

	return TierReadingsFromSeries(fastSeries, mediumSeries, slowSeries)
}

/*
Reset clears contagion spread history.
*/
func (multiverse *Multiverse) Reset() error {
	if multiverse == nil {
		return nil
	}

	multiverse.contagion = NewContagion(multiverse.contagion.config)

	return nil
}

func (multiverse *Multiverse) snapshots() []WindowSnapshot {
	if multiverse == nil {
		return nil
	}

	snapshots := make([]WindowSnapshot, 0, len(multiverse.windowSets))

	for _, windowSet := range multiverse.windowSets {
		if windowSet == nil {
			continue
		}

		snapshot := windowSet.Snapshot(multiverse.tiers)

		if snapshot.Fast == nil && snapshot.Medium == nil && snapshot.Slow == nil {
			continue
		}

		snapshots = append(snapshots, snapshot)
	}

	return snapshots
}
