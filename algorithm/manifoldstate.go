package algorithm

import (
	"math"

	"github.com/theapemachine/datura"
)

const manifoldstatePayloadFields = 5

/*
ManifoldstateOutcome holds herd, shock, drift, and noise scores for the 3D manifold field.
*/
type ManifoldstateOutcome struct {
	HerdScore   float64
	ShockScore  float64
	DriftScore  float64
	NoiseScore  float64
	Strength    float64
	Category    int
	Eligible    bool
	Price       float64
	Spread      float64
}

/*
Manifoldstate classifies systemic herd, liquidity shock, synchronized drift, and stochastic noise.

Payload layout: pressureGradNorm, coherenceMag2, guidanceSpeed, viscosityProxy, price.
*/
type Manifoldstate struct {
	artifact *datura.Artifact
	outcome  ManifoldstateOutcome
}

/*
NewManifoldstate returns a manifold-state stage for io.ReadWriter pipelines.
*/
func NewManifoldstate() *Manifoldstate {
	return &Manifoldstate{
		artifact: datura.Acquire("manifoldstate", datura.Artifact_Type_json),
	}
}

func (manifoldstate *Manifoldstate) Write(p []byte) (int, error) {
	return manifoldstate.artifact.Write(p)
}

func (manifoldstate *Manifoldstate) Read(p []byte) (int, error) {
	rehydrateArtifact(&manifoldstate.artifact, "manifoldstate", datura.Artifact_Type_json)

	payload, err := manifoldstate.artifact.Payload()

	if err == nil {
		manifoldstate.outcome = manifoldstate.evaluate(payloadSamples(payload))
		manifoldstate.publishReadings()
	}

	return manifoldstate.artifact.Read(p)
}

func (manifoldstate *Manifoldstate) Close() error {
	return nil
}

/*
Outcome returns scores from the last Read.
*/
func (manifoldstate *Manifoldstate) Outcome() ManifoldstateOutcome {
	return manifoldstate.outcome
}

func (manifoldstate *Manifoldstate) evaluate(batch []float64) ManifoldstateOutcome {
	if len(batch) < manifoldstatePayloadFields {
		return ManifoldstateOutcome{}
	}

	pressureGradNorm := batch[0]
	coherenceMag2 := batch[1]
	guidanceSpeed := batch[2]
	viscosityProxy := batch[3]
	price := batch[4]

	if price <= 0 {
		return ManifoldstateOutcome{}
	}

	if coherenceMag2 <= 0 || guidanceSpeed <= 0 || viscosityProxy <= 0 {
		return ManifoldstateOutcome{}
	}

	if math.IsNaN(pressureGradNorm) || math.IsInf(pressureGradNorm, 0) ||
		math.IsNaN(coherenceMag2) || math.IsInf(coherenceMag2, 0) ||
		math.IsNaN(guidanceSpeed) || math.IsInf(guidanceSpeed, 0) ||
		math.IsNaN(viscosityProxy) || math.IsInf(viscosityProxy, 0) {
		return ManifoldstateOutcome{}
	}

	herdScore := coherenceMag2 * guidanceSpeed
	shockScore := pressureGradNorm
	driftScore := guidanceSpeed * (1 / math.Max(viscosityProxy, 1e-9))
	noiseScore := viscosityProxy * (1 - coherenceMag2)

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
		return ManifoldstateOutcome{}
	}

	spread := viscosityProxy

	return ManifoldstateOutcome{
		HerdScore:  herdScore,
		ShockScore: shockScore,
		DriftScore: driftScore,
		NoiseScore: noiseScore,
		Strength:   strength,
		Category:   category,
		Eligible:   true,
		Price:      price,
		Spread:     spread,
	}
}

func (manifoldstate *Manifoldstate) publishReadings() {
	pokeFloat(manifoldstate.artifact, "manifoldstate.herd", manifoldstate.outcome.HerdScore)
	pokeFloat(manifoldstate.artifact, "manifoldstate.shock", manifoldstate.outcome.ShockScore)
	pokeFloat(manifoldstate.artifact, "manifoldstate.drift", manifoldstate.outcome.DriftScore)
	pokeFloat(manifoldstate.artifact, "manifoldstate.noise", manifoldstate.outcome.NoiseScore)
	pokeFloat(manifoldstate.artifact, "manifoldstate.strength", manifoldstate.outcome.Strength)
}

func (manifoldstate *Manifoldstate) HerdReading() *ManifoldstateReading {
	return newManifoldstateReading(manifoldstate, func(outcome ManifoldstateOutcome) float64 {
		return outcome.HerdScore
	})
}

func (manifoldstate *Manifoldstate) ShockReading() *ManifoldstateReading {
	return newManifoldstateReading(manifoldstate, func(outcome ManifoldstateOutcome) float64 {
		return outcome.ShockScore
	})
}

func (manifoldstate *Manifoldstate) DriftReading() *ManifoldstateReading {
	return newManifoldstateReading(manifoldstate, func(outcome ManifoldstateOutcome) float64 {
		return outcome.DriftScore
	})
}

func (manifoldstate *Manifoldstate) NoiseReading() *ManifoldstateReading {
	return newManifoldstateReading(manifoldstate, func(outcome ManifoldstateOutcome) float64 {
		return outcome.NoiseScore
	})
}

type ManifoldstateReading struct {
	artifact      *datura.Artifact
	manifoldstate *Manifoldstate
	project       func(ManifoldstateOutcome) float64
}

func newManifoldstateReading(
	manifoldstate *Manifoldstate,
	project func(ManifoldstateOutcome) float64,
) *ManifoldstateReading {
	return &ManifoldstateReading{
		artifact:      datura.Acquire("manifoldstate-reading", datura.Artifact_Type_json),
		manifoldstate: manifoldstate,
		project:       project,
	}
}

func (reading *ManifoldstateReading) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (reading *ManifoldstateReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.manifoldstate != nil && reading.project != nil {
		value = reading.project(reading.manifoldstate.outcome)
	}

	_ = reading.artifact.SetPayload(encodePayload(value))

	return reading.artifact.Read(p)
}

func (reading *ManifoldstateReading) Close() error {
	return nil
}
