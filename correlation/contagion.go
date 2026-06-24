package correlation

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
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
