package equation

import (
	"math"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/probability"
)

/*
Manifoldstate classifies systemic herd, liquidity shock, synchronized drift, and stochastic noise.
*/
type Manifoldstate struct{}

/*
NewManifoldstate returns a typed manifold-state classifier.
*/
func NewManifoldstate() *Manifoldstate {
	return &Manifoldstate{}
}

/*
ManifoldstateOutput carries typed category evidence.
*/
type ManifoldstateOutput struct {
	HerdScore  float64
	ShockScore float64
	DriftScore float64
	NoiseScore float64
	Strength   float64
	Category   int
	Eligible   bool
}

/*
Measure classifies manifold features using their semantic feature schema.
*/
func (manifoldstate *Manifoldstate) Measure(frame FeatureFrame) (ManifoldstateOutput, error) {
	outcome := evaluateManifoldstate(frame, frame.Inputs)

	if !outcome.Eligible || outcome.Strength <= 0 {
		return ManifoldstateOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"equation: invalid stage input",
			nil,
		))
	}

	return outcome, nil
}

func evaluateManifoldstate(frame FeatureFrame, inputKeys []string) ManifoldstateOutput {
	fields, err := FeatureFields(frame, inputKeys)

	if err != nil || len(fields) < len(ManifoldInputKeys) {
		return ManifoldstateOutput{}
	}

	pressureGradNorm := fields[0]
	coherenceMag2 := fields[1]
	guidanceSpeed := fields[2]
	viscosityProxy := fields[3]
	price := fields[4]

	if price <= 0 {
		return ManifoldstateOutput{}
	}

	if coherenceMag2 <= 0 || guidanceSpeed <= 0 || viscosityProxy <= 0 {
		return ManifoldstateOutput{}
	}

	if math.IsNaN(pressureGradNorm) || math.IsInf(pressureGradNorm, 0) ||
		math.IsNaN(coherenceMag2) || math.IsInf(coherenceMag2, 0) ||
		math.IsNaN(guidanceSpeed) || math.IsInf(guidanceSpeed, 0) ||
		math.IsNaN(viscosityProxy) || math.IsInf(viscosityProxy, 0) {
		return ManifoldstateOutput{}
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
			return ManifoldstateOutput{}
		}
	}

	if shockRaw > 0 {
		shockScore, err = probability.MagnitudeMargin(shockRaw)

		if err != nil {
			return ManifoldstateOutput{}
		}
	}

	if driftRaw > 0 {
		driftScore, err = probability.MagnitudeMargin(driftRaw)

		if err != nil {
			return ManifoldstateOutput{}
		}
	}

	if noiseRaw > 0 {
		noiseScore, err = probability.MagnitudeMargin(noiseRaw)

		if err != nil {
			return ManifoldstateOutput{}
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
		return ManifoldstateOutput{}
	}

	strength := best

	if strength <= 0 || math.IsNaN(strength) || math.IsInf(strength, 0) {
		return ManifoldstateOutput{}
	}

	return ManifoldstateOutput{
		HerdScore:  herdScore,
		ShockScore: shockScore,
		DriftScore: driftScore,
		NoiseScore: noiseScore,
		Strength:   strength,
		Category:   category,
		Eligible:   true,
	}
}
