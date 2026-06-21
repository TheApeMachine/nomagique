package equation

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
)

const manifoldstatePayloadFields = 5

/*
Manifoldstate classifies systemic herd, liquidity shock, synchronized drift, and stochastic noise.

Payload layout: pressureGradNorm, coherenceMag2, guidanceSpeed, viscosityProxy, price.
*/
type Manifoldstate struct {
	bytes []byte
}

/*
NewManifoldstate returns a manifold-state stage.
*/
func NewManifoldstate() io.ReadWriteCloser {
	return &Manifoldstate{}
}

func (manifoldstate *Manifoldstate) Write(p []byte) (int, error) {
	manifoldstate.bytes = append(manifoldstate.bytes[:0], p...)

	return len(p), nil
}

func (manifoldstate *Manifoldstate) Read(p []byte) (int, error) {
	state, err := stageState(manifoldstate.bytes)

	if err != nil {
		return 0, err
	}

	batch := Features(state)
	outcome := evaluateManifoldstate(batch)

	if !outcome.eligible || outcome.strength <= 0 {
		return emitZero(state, p)
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

func evaluateManifoldstate(batch []float64) manifoldstateOutcome {
	if len(batch) < manifoldstatePayloadFields {
		return manifoldstateOutcome{}
	}

	pressureGradNorm := batch[0]
	coherenceMag2 := batch[1]
	guidanceSpeed := batch[2]
	viscosityProxy := batch[3]
	price := batch[4]

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
