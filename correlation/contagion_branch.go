package correlation

import (
	"math"
	"strconv"

	"github.com/theapemachine/datura"
)

type contagionBranch struct {
	lastLevel float64
	lastEpoch float64
	starts    []float64
	ends      []float64
	rets      []float64
	member    string
}

func (contagion *Contagion) memberSegment(member int) string {
	return strconv.Itoa(member)
}

func (contagion *Contagion) branchPath(member string) []any {
	return []any{"interval", member}
}

func (contagion *Contagion) preserveBranch(member string) contagionBranch {
	base := contagion.branchPath(member)

	return contagionBranch{
		lastLevel: datura.Peek[float64](contagion.artifact, append(base, "lastLevel")...),
		lastEpoch: datura.Peek[float64](contagion.artifact, append(base, "lastEpoch")...),
		starts:    datura.Peek[[]float64](contagion.artifact, append(base, "starts")...),
		ends:      datura.Peek[[]float64](contagion.artifact, append(base, "ends")...),
		rets:      datura.Peek[[]float64](contagion.artifact, append(base, "rets")...),
		member:    member,
	}
}

func (contagion *Contagion) restoreBranch(preserved contagionBranch) {
	if preserved.lastLevel <= 0 && preserved.lastEpoch <= 0 && len(preserved.rets) == 0 {
		return
	}

	base := contagion.branchPath(preserved.member)
	contagion.artifact.Poke(preserved.lastLevel, append(base, "lastLevel")...)
	contagion.artifact.Poke(preserved.lastEpoch, append(base, "lastEpoch")...)
	contagion.artifact.Poke(preserved.starts, append(base, "starts")...)
	contagion.artifact.Poke(preserved.ends, append(base, "ends")...)
	contagion.artifact.Poke(preserved.rets, append(base, "rets")...)
}

func (contagion *Contagion) ingest(capacity int, epoch int64, level float64, member string) {
	if capacity <= 0 {
		capacity = contagion.memberCapacityFromArtifact()
	}

	if level <= 0 {
		return
	}

	base := contagion.branchPath(member)
	lastLevel := datura.Peek[float64](contagion.artifact, append(base, "lastLevel")...)
	lastEpoch := int64(datura.Peek[float64](contagion.artifact, append(base, "lastEpoch")...))

	if lastLevel <= 0 || lastEpoch <= 0 {
		contagion.artifact.Poke(level, append(base, "lastLevel")...)
		contagion.artifact.Poke(float64(epoch), append(base, "lastEpoch")...)

		return
	}

	if epoch <= lastEpoch {
		contagion.artifact.Poke(level, append(base, "lastLevel")...)

		return
	}

	starts := datura.Peek[[]float64](contagion.artifact, append(base, "starts")...)
	ends := datura.Peek[[]float64](contagion.artifact, append(base, "ends")...)
	rets := datura.Peek[[]float64](contagion.artifact, append(base, "rets")...)

	starts = append(starts, float64(lastEpoch))
	ends = append(ends, float64(epoch))
	rets = append(rets, math.Log(level/lastLevel))

	if len(starts) > capacity {
		trim := len(starts) - capacity
		starts = starts[trim:]
		ends = ends[trim:]
		rets = rets[trim:]
	}

	contagion.artifact.Poke(starts, append(base, "starts")...)
	contagion.artifact.Poke(ends, append(base, "ends")...)
	contagion.artifact.Poke(rets, append(base, "rets")...)
	contagion.artifact.Poke(level, append(base, "lastLevel")...)
	contagion.artifact.Poke(float64(epoch), append(base, "lastEpoch")...)
}

func (contagion *Contagion) tailBranch(window int, member string) (starts, ends, rets []float64) {
	base := contagion.branchPath(member)
	starts = append([]float64(nil), datura.Peek[[]float64](contagion.artifact, append(base, "starts")...)...)
	ends = append([]float64(nil), datura.Peek[[]float64](contagion.artifact, append(base, "ends")...)...)
	rets = append([]float64(nil), datura.Peek[[]float64](contagion.artifact, append(base, "rets")...)...)

	if window <= 0 {
		return nil, nil, nil
	}

	if len(starts) > window {
		trim := len(starts) - window
		starts = starts[trim:]
		ends = ends[trim:]
		rets = rets[trim:]
	}

	return starts, ends, rets
}

func (contagion *Contagion) recordMember(member int) {
	if member <= 0 {
		return
	}

	memberIDs := datura.Peek[[]float64](contagion.artifact, "member", "ids")

	for _, existing := range memberIDs {
		if int(existing) == member {
			return
		}
	}

	memberIDs = append(memberIDs, float64(member))
	contagion.artifact.Poke(memberIDs, "member", "ids")
}

func (contagion *Contagion) memberIDsFromConfig() []int {
	raw := datura.Peek[[]float64](contagion.artifact, "member", "ids")
	memberIDs := make([]int, 0, len(raw))

	for _, value := range raw {
		memberID := int(value)

		if memberID <= 0 {
			continue
		}

		memberIDs = append(memberIDs, memberID)
	}

	return memberIDs
}
