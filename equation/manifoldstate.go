package equation

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/probability"
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

	inputKeys := EnsureFeatureSchema(state, manifoldstate.artifact, ManifoldInputKeys)
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
	fields, err := FeatureFields(state, inputKeys)

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

	herdRaw := coherenceMag2 * guidanceSpeed
	shockRaw := pressureGradNorm
	driftRaw := guidanceSpeed / viscosityProxy
	noiseRaw := viscosityProxy * math.Max(0, 1-coherenceMag2)

	herdScore := 0.0
	shockScore := 0.0
	driftScore := 0.0
	noiseScore := 0.0

	if herdRaw > 0 {
		herdScore, err = probability.MagnitudeMargin(herdRaw)

		if err != nil {
			return manifoldstateOutcome{}
		}
	}

	if shockRaw > 0 {
		shockScore, err = probability.MagnitudeMargin(shockRaw)

		if err != nil {
			return manifoldstateOutcome{}
		}
	}

	if driftRaw > 0 {
		driftScore, err = probability.MagnitudeMargin(driftRaw)

		if err != nil {
			return manifoldstateOutcome{}
		}
	}

	if noiseRaw > 0 {
		noiseScore, err = probability.MagnitudeMargin(noiseRaw)

		if err != nil {
			return manifoldstateOutcome{}
		}
	}

	best := math.Max(herdScore, math.Max(shockScore, math.Max(driftScore, noiseScore)))
	category := 0
	winners := 0

	for index, score := range []float64{herdScore, shockScore, driftScore, noiseScore} {
		if score != best {
			continue
		}

		winners++
		category = index + 1
	}

	if winners != 1 {
		return manifoldstateOutcome{}
	}

	strength := best

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
