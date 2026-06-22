package equation

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
)

/*
Manifoldstate classifies systemic herd, liquidity shock, synchronized drift, and stochastic noise.
The constructor artifact holds schema inputs; Write buffers inbound wire on its payload.
*/
type Manifoldstate struct {
	artifact *datura.Artifact
}

/*
NewManifoldstate returns a manifold-state stage wired from config attributes.
*/
func NewManifoldstate(artifact *datura.Artifact) io.ReadWriteCloser {
	if artifact == nil {
		artifact = datura.Acquire("manifoldstate", datura.APPJSON)
	}

	if len(datura.Peek[[]string](artifact, "inputs")) == 0 {
		artifact.Poke(ManifoldInputKeys, "inputs")
	}

	return &Manifoldstate{
		artifact: artifact,
	}
}

func (manifoldstate *Manifoldstate) Write(p []byte) (int, error) {
	manifoldstate.artifact.WithPayload(p)
	return len(p), nil
}

func (manifoldstate *Manifoldstate) Read(p []byte) (int, error) {
	state, err := stageState(manifoldstate.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	inputKeys := ensureFeatureSchema(state, manifoldstate.artifact, ManifoldInputKeys)
	outcome := evaluateManifoldstate(state, inputKeys)

	if !outcome.eligible || outcome.strength <= 0 {
		return rejectStage(state, "equation: invalid stage input")
	}

	return emitOutput(state, p, datura.Map[float64]{
		"value":      outcome.strength,
		"herdScore":  outcome.herdScore,
		"shockScore": outcome.shockScore,
		"driftScore": outcome.driftScore,
		"noiseScore": outcome.noiseScore,
		"category":   float64(outcome.category),
	})
}

func (manifoldstate *Manifoldstate) Close() error {
	return nil
}

type manifoldstateOutcome struct {
	herdScore  float64
	shockScore float64
	driftScore float64
	noiseScore float64
	strength   float64
	category   int
	eligible   bool
}

func evaluateManifoldstate(state *datura.Artifact, inputKeys []string) manifoldstateOutcome {
	fields, err := featureFields(state, inputKeys)

	if err != nil || len(fields) < len(ManifoldInputKeys) {
		return manifoldstateOutcome{}
	}

	pressureGradNorm := fields[0]
	coherenceMag2 := fields[1]
	guidanceSpeed := fields[2]
	viscosityProxy := fields[3]
	price := fields[4]

	if price <= 0 {
		return manifoldstateOutcome{}
	}

	if coherenceMag2 <= 0 || guidanceSpeed <= 0 || viscosityProxy <= 0 {
		return manifoldstateOutcome{}
	}

	if math.IsNaN(pressureGradNorm) || math.IsInf(pressureGradNorm, 0) ||
		math.IsNaN(coherenceMag2) || math.IsInf(coherenceMag2, 0) ||
		math.IsNaN(guidanceSpeed) || math.IsInf(guidanceSpeed, 0) ||
		math.IsNaN(viscosityProxy) || math.IsInf(viscosityProxy, 0) {
		return manifoldstateOutcome{}
	}

	herdScore := coherenceMag2 * guidanceSpeed
	shockScore := pressureGradNorm
	driftScore := guidanceSpeed / viscosityProxy
	noiseScore := viscosityProxy * math.Max(0, 1-coherenceMag2)

	best := noiseScore
	category := 4

	if herdScore > best && coherenceMag2 > 0 {
		best = herdScore
		category = 1
	}

	if shockScore > best {
		best = shockScore
		category = 2
	}

	if driftScore > best {
		best = driftScore
		category = 3
	}

	strength := shockScore

	switch category {
	case 1:
		strength = herdScore
	case 2:
		strength = shockScore
	case 3:
		strength = driftScore
	case 4:
		strength = noiseScore
	}

	if strength <= 0 || math.IsNaN(strength) || math.IsInf(strength, 0) {
		return manifoldstateOutcome{}
	}

	return manifoldstateOutcome{
		herdScore:  herdScore,
		shockScore: shockScore,
		driftScore: driftScore,
		noiseScore: noiseScore,
		strength:   strength,
		category:   category,
		eligible:   true,
	}
}
