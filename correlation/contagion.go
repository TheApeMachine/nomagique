package correlation

import (
	"math"
	"strconv"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/statistic"
)

/*
Contagion estimates ensemble coupling from multi-tier interval snapshots and adapts
the published reading from fast-medium-slow spread dynamics.

Write member, sample, and paired on each inbound artifact to accumulate per-member
interval history. Tier and estimator config live on config.* attributes.
*/
type Contagion struct {
	artifact *datura.Artifact
}

type contagionBranch struct {
	lastLevel float64
	lastEpoch float64
	starts    []float64
	ends      []float64
	rets      []float64
	member    string
}

/*
NewContagion creates an ensemble coupling stage wired from config attributes on the artifact.
*/
func NewContagion(artifact *datura.Artifact) *Contagion {
	return &Contagion{
		artifact: artifact,
	}
}

func (contagion *Contagion) Read(p []byte) (int, error) {
	if contagion == nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute contagion",
			ContagionError(ContagionErrorNilReceiver),
		))
	}

	state := datura.Acquire("contagion-state", datura.APPJSON)

	if _, err := state.Write(contagion.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"contagion: state write failed",
			err,
		))
	}

	memberField := datura.Peek[string](contagion.artifact, "memberKey")

	if memberField == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"contagion: memberKey required",
			nil,
		))
	}

	sampleField := datura.Peek[string](contagion.artifact, "sampleKey")

	if sampleField == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"contagion: sampleKey required",
			nil,
		))
	}

	pairedField := datura.Peek[string](contagion.artifact, "pairedKey")

	if pairedField == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"contagion: pairedKey required",
			nil,
		))
	}

	wireRoot := datura.Peek[string](state, "root")

	if wireRoot == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"contagion: root required",
			nil,
		))
	}

	wireInputs := datura.Peek[[]string](state, "inputs")

	if len(wireInputs) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"contagion: inputs required",
			nil,
		))
	}

	var memberValue float64
	memberFound := false

	for wireIndex, wireInput := range wireInputs {
		if wireInput != memberField {
			continue
		}

		if wireRoot == "features" {
			features := datura.Peek[[]float64](state, wireRoot)

			if wireIndex >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"contagion: feature index out of range",
					nil,
				))
			}

			memberValue = features[wireIndex]
		}

		if wireRoot != "features" {
			memberValue = datura.Peek[float64](state, wireRoot, wireInput)
		}

		memberFound = true
	}

	if !memberFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"contagion: memberKey not in inputs",
			nil,
		))
	}

	var levelValue float64
	levelFound := false

	for wireIndex, wireInput := range wireInputs {
		if wireInput != pairedField {
			continue
		}

		if wireRoot == "features" {
			features := datura.Peek[[]float64](state, wireRoot)

			if wireIndex >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"contagion: feature index out of range",
					nil,
				))
			}

			levelValue = features[wireIndex]
		}

		if wireRoot != "features" {
			levelValue = datura.Peek[float64](state, wireRoot, wireInput)
		}

		levelFound = true
	}

	if !levelFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"contagion: pairedKey not in inputs",
			nil,
		))
	}

	var epochValue float64
	epochFound := false

	for wireIndex, wireInput := range wireInputs {
		if wireInput != sampleField {
			continue
		}

		if wireRoot == "features" {
			features := datura.Peek[[]float64](state, wireRoot)

			if wireIndex >= len(features) {
				return 0, errnie.Error(errnie.Err(
					errnie.Validation,
					"contagion: feature index out of range",
					nil,
				))
			}

			epochValue = features[wireIndex]
		}

		if wireRoot != "features" {
			epochValue = datura.Peek[float64](state, wireRoot, wireInput)
		}

		epochFound = true
	}

	if !epochFound {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"contagion: sampleKey not in inputs",
			nil,
		))
	}

	member := int(memberValue)
	level := levelValue

	if member > 0 && level > 0 {
		epoch := int64(epochValue)
		contagion.recordMember(member)
		contagion.ingest(
			contagion.memberCapacityFromArtifact(),
			epoch,
			level,
			contagion.memberSegment(member),
		)
	}

	snapshots := contagion.snapshots()

	if len(snapshots) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation, "unable to compute contagion",
			ContagionError(ContagionErrorInsufficientSnapshots),
		))
	}

	output, readings, err := contagion.observeSnapshots(snapshots)

	if err != nil {
		return 0, err
	}

	state.MergeOutput("value", output)
	state.MergeOutput("tier.fast", readings.fast)
	state.MergeOutput("tier.medium", readings.medium)
	state.MergeOutput("tier.slow", readings.slow)
	state.Poke("output", "root")
	state.Poke([]string{"value", "tier.fast", "tier.medium", "tier.slow"}, "inputs")
	return state.Read(p)
}

func (contagion *Contagion) Write(p []byte) (int, error) {
	reset := inboundReset(p)
	memberIDs := contagion.memberIDsFromConfig()
	preservedMembers := make([]contagionBranch, 0, len(memberIDs))

	for _, member := range memberIDs {
		preservedMembers = append(preservedMembers, contagion.preserveBranch(contagion.memberSegment(member)))
	}

	spreadValues := datura.Peek[[]float64](contagion.artifact, "spread", "values")
	spreadHead := datura.Peek[float64](contagion.artifact, "spread", "head")
	spreadCount := datura.Peek[float64](contagion.artifact, "spread", "count")
	memberIDList := datura.Peek[[]float64](contagion.artifact, "member", "ids")

	contagion.artifact.WithPayload(p)

	if reset {
		return len(p), nil
	}

	for _, preserved := range preservedMembers {
		contagion.restoreBranch(preserved)
	}

	if len(memberIDList) > 0 {
		contagion.artifact.Poke(memberIDList, "member", "ids")
	}

	if len(spreadValues) > 0 {
		contagion.artifact.Poke(spreadValues, "spread", "values")
		contagion.artifact.Poke(spreadHead, "spread", "head")
		contagion.artifact.Poke(spreadCount, "spread", "count")
	}

	return len(p), nil
}

func (contagion *Contagion) Close() error {
	return nil
}

type tierWindows struct {
	fast   int
	medium int
	slow   int
}

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
	fast, slow, err := statistic.NewRollingWindow(tiers.fast, tiers.slow).Resolve(make([]float64, maxLen))

	if err != nil {
		if maxLen <= 0 {
			return tiers
		}

		fast = maxLen / 3

		if fast < 1 {
			fast = 1
		}

		slow = maxLen
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

func pushSpread(artifact *datura.Artifact, capacity int, value float64) {
	if capacity <= 0 {
		capacity = 1
	}

	values := datura.Peek[[]float64](artifact, "spread", "values")
	head := int(datura.Peek[float64](artifact, "spread", "head"))
	count := int(datura.Peek[float64](artifact, "spread", "count"))

	if len(values) != capacity {
		values = make([]float64, capacity)
		head = 0
		count = 0
	}

	values[head] = value
	head = (head + 1) % capacity

	if count < capacity {
		count++
	}

	artifact.Poke(values, "spread", "values")
	artifact.Poke(float64(head), "spread", "head")
	artifact.Poke(float64(count), "spread", "count")
}

func spreadAt(artifact *datura.Artifact, index int) float64 {
	values := datura.Peek[[]float64](artifact, "spread", "values")
	head := int(datura.Peek[float64](artifact, "spread", "head"))
	count := int(datura.Peek[float64](artifact, "spread", "count"))
	capacity := len(values)

	if index < 0 || index >= count || capacity == 0 {
		return 0
	}

	start := 0

	if count >= capacity {
		start = head
	}

	return values[(start+index)%capacity]
}

func spreadLength(artifact *datura.Artifact) int {
	return int(datura.Peek[float64](artifact, "spread", "count"))
}

func adaptiveSpreadThreshold(
	artifact *datura.Artifact,
	slowBaseline float64,
	sigma float64,
) float64 {
	count := spreadLength(artifact)

	if count < 4 {
		if slowBaseline > 0 {
			return slowBaseline
		}

		return 0
	}

	mean := 0.0

	for index := 0; index < count; index++ {
		mean += spreadAt(artifact, index)
	}

	mean /= float64(count)

	if count < 2 {
		return mean
	}

	variance := 0.0

	for index := 0; index < count; index++ {
		delta := spreadAt(artifact, index) - mean
		variance += delta * delta
	}

	stddev := math.Sqrt(variance / float64(count-1))
	floor := mean * mean / (mean + slowBaseline)

	if stddev <= 0 {
		return math.Max(floor, mean)
	}

	return math.Max(floor, mean+sigma*stddev)
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

type ContagionErrorType string

const (
	ContagionErrorNilReceiver           ContagionErrorType = "require non-nil contagion stage"
	ContagionErrorInsufficientSnapshots ContagionErrorType = "require member interval snapshots"
)

type ContagionError string

func (contagionError ContagionError) Error() string {
	return string(contagionError)
}
