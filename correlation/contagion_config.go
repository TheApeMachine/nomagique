package correlation

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

type tierWindows struct {
	fast   int
	medium int
	slow   int
}

func (contagion *Contagion) tiersFromArtifact() tierWindows {
	tiers := tierWindows{
		fast:   int(datura.Peek[float64](contagion.artifact, "config", "tier", "fast")),
		medium: int(datura.Peek[float64](contagion.artifact, "config", "tier", "medium")),
		slow:   int(datura.Peek[float64](contagion.artifact, "config", "tier", "slow")),
	}

	if tiers.fast > 0 && tiers.medium > 0 && tiers.slow > 0 {
		return tiers
	}

	maxLen := contagion.maxMemberRetLength()
	fast, slow, err := statistic.ResolveWindows(make([]float64, maxLen), tiers.fast, tiers.slow)

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"correlation contagion: tier windows resolution failed",
			err,
		))

		return tierWindows{}
	}

	medium := (fast + slow) / 2

	if medium <= fast {
		medium = fast + 1
	}

	if slow <= medium {
		slow = medium + 1
	}

	return tierWindows{fast: fast, medium: medium, slow: slow}
}

func (contagion *Contagion) maxMemberRetLength() int {
	maxLen := 0

	for _, member := range contagion.memberIDsFromConfig() {
		rets := datura.Peek[[]float64](
			contagion.artifact,
			append(contagion.branchPath(contagion.memberSegment(member)), "rets")...,
		)

		if len(rets) > maxLen {
			maxLen = len(rets)
		}
	}

	return maxLen
}

func (contagion *Contagion) minSamplesFromArtifact() int {
	minSamples := int(datura.Peek[float64](contagion.artifact, "config", "minSamples"))

	if minSamples > 0 {
		return minSamples
	}

	tiers := contagion.tiersFromArtifact()
	derived := tiers.slow / 2

	if derived < 2 {
		derived = 2
	}

	return derived
}

func (contagion *Contagion) memberCapFromArtifact() int {
	memberCap := int(datura.Peek[float64](contagion.artifact, "config", "memberCap"))

	if memberCap > 0 {
		return memberCap
	}

	memberCap = len(contagion.memberIDsFromConfig())

	if memberCap <= 0 {
		memberCap = 1
	}

	return memberCap
}

func (contagion *Contagion) adaptiveSigmaFromArtifact() (float64, error) {
	if datura.Peek[string](contagion.artifact, "config", "adaptiveSigma") != "" ||
		datura.Peek[float64](contagion.artifact, "config", "adaptiveSigma") != 0 {
		return datura.Peek[float64](contagion.artifact, "config", "adaptiveSigma"), nil
	}

	count := spreadLength(contagion.artifact)

	if count < 2 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"correlation contagion: insufficient spread history for adaptiveSigma",
			nil,
		))
	}

	mean := 0.0

	for index := 0; index < count; index++ {
		mean += spreadAt(contagion.artifact, index)
	}

	mean /= float64(count)

	if mean == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"correlation contagion: spread mean is zero",
			nil,
		))
	}

	variance := 0.0

	for index := 0; index < count; index++ {
		delta := spreadAt(contagion.artifact, index) - mean
		variance += delta * delta
	}

	stddev := math.Sqrt(variance / float64(count-1))

	if stddev <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"correlation contagion: spread stddev is invalid",
			nil,
		))
	}

	return stddev / math.Abs(mean), nil
}

func (contagion *Contagion) spreadCapacityFromArtifact() int {
	capacity := int(datura.Peek[float64](contagion.artifact, "config", "spreadCapacity"))

	if capacity > 0 {
		return capacity
	}

	capacity = contagion.memberCapFromArtifact() * 4

	if capacity < 8 {
		capacity = 8
	}

	return capacity
}

func (contagion *Contagion) memberCapacityFromArtifact() int {
	capacity := int(datura.Peek[float64](contagion.artifact, "config", "capacity"))

	if capacity > 0 {
		return capacity
	}

	capacity = contagion.maxMemberRetLength() * 2

	if capacity < 8 {
		capacity = 8
	}

	return capacity
}
