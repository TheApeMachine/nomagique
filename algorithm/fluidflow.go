package algorithm

import (
	"math"

	"github.com/theapemachine/datura"
)

const fluidflowPayloadFields = 14

/*
FluidflowOutcome holds Reynolds-classification scores for book microfluidics.
*/
type FluidflowOutcome struct {
	LaminarScore   float64
	TurbulentScore float64
	InertialScore  float64
	ViscousScore   float64
	Strength       float64
	Category       int
	Eligible       bool
	Price          float64
	SpreadBPS      float64
	ChangePct      float64
	Volume         float64
}

/*
Fluidflow classifies laminar, turbulent, inertial, and viscous book-flow regimes.

Payload layout: reynolds, absDivergence, viscosity, midAddRate, midExecuteRate,
laminarCeiling, turbulentFloor, turbulentReady, divergenceEdge, icebergScore,
price, spreadBPS, changePct, volume.
*/
type Fluidflow struct {
	artifact *datura.Artifact
	outcome  FluidflowOutcome
}

/*
NewFluidflow returns a fluid-dynamics stage for io.ReadWriter pipelines.
*/
func NewFluidflow() *Fluidflow {
	return &Fluidflow{
		artifact: datura.Acquire("fluidflow", datura.Artifact_Type_json),
	}
}

func (fluidflow *Fluidflow) Write(p []byte) (int, error) {
	return fluidflow.artifact.Write(p)
}

func (fluidflow *Fluidflow) Read(p []byte) (int, error) {
	rehydrateArtifact(&fluidflow.artifact, "fluidflow", datura.Artifact_Type_json)

	payload, err := fluidflow.artifact.Payload()

	if err == nil {
		fluidflow.outcome = fluidflow.evaluate(payloadSamples(payload))
		fluidflow.publishReadings()
	}

	return fluidflow.artifact.Read(p)
}

func (fluidflow *Fluidflow) Close() error {
	return nil
}

/*
Outcome returns scores from the last Read.
*/
func (fluidflow *Fluidflow) Outcome() FluidflowOutcome {
	return fluidflow.outcome
}

func (fluidflow *Fluidflow) evaluate(batch []float64) FluidflowOutcome {
	if len(batch) < fluidflowPayloadFields {
		return FluidflowOutcome{}
	}

	reynolds := batch[0]
	divergence := batch[1]
	viscosity := batch[2]
	_ = batch[3]
	_ = batch[4]
	laminarCeiling := batch[5]
	turbulentFloor := batch[6]
	turbulentReady := batch[7] > 0
	divergenceEdge := batch[8]
	icebergScore := batch[9]
	price := batch[10]
	spreadBPS := batch[11]
	changePct := batch[12]
	volume := batch[13]

	if price <= 0 || spreadBPS <= 0 || changePct <= 0 || volume <= 0 {
		return FluidflowOutcome{}
	}

	if viscosity <= 0 || math.IsNaN(reynolds) || math.IsInf(reynolds, 0) {
		return FluidflowOutcome{}
	}

	if divergenceEdge <= 0 && divergence > 0 {
		divergenceEdge = divergence
	}

	laminarScore := 0.0

	if reynolds < laminarCeiling && divergenceEdge > 0 && divergence < divergenceEdge {
		laminarScore = viscosity * (1 - divergence/divergenceEdge)
	}

	turbulentScore := 0.0

	if turbulentReady && reynolds >= turbulentFloor {
		turbulentScore = reynolds / turbulentFloor
	}

	inertialScore := divergence

	viscousScore := 0.0

	if viscosity > 0 {
		viscousScore = divergence / viscosity
	}

	if icebergScore > 0 {
		viscousScore = math.Max(viscousScore, icebergScore)
	}

	category := 1
	best := laminarScore

	if turbulentScore > best {
		best = turbulentScore
		category = 2
	}

	if inertialScore > best {
		best = inertialScore
		category = 3
	}

	if viscousScore > best {
		best = viscousScore
		category = 4
	}

	if best <= 0 && price > 0 && reynolds < laminarCeiling {
		category = 1
		laminarScore = viscosity
		best = laminarScore
	}

	strength := reynolds

	if strength <= 0 || math.IsNaN(strength) || math.IsInf(strength, 0) {
		strength = math.Max(
			laminarScore,
			math.Max(turbulentScore, math.Max(inertialScore, viscousScore)),
		)
	}

	if strength <= 0 || math.IsNaN(strength) || math.IsInf(strength, 0) {
		return FluidflowOutcome{}
	}

	return FluidflowOutcome{
		LaminarScore:   laminarScore,
		TurbulentScore: turbulentScore,
		InertialScore:  inertialScore,
		ViscousScore:   viscousScore,
		Strength:       strength,
		Category:       category,
		Eligible:       true,
		Price:          price,
		SpreadBPS:      spreadBPS,
		ChangePct:      changePct,
		Volume:         volume,
	}
}

func (fluidflow *Fluidflow) publishReadings() {
	pokeFloat(fluidflow.artifact, "fluidflow.laminar", fluidflow.outcome.LaminarScore)
	pokeFloat(fluidflow.artifact, "fluidflow.turbulent", fluidflow.outcome.TurbulentScore)
	pokeFloat(fluidflow.artifact, "fluidflow.inertial", fluidflow.outcome.InertialScore)
	pokeFloat(fluidflow.artifact, "fluidflow.viscous", fluidflow.outcome.ViscousScore)
	pokeFloat(fluidflow.artifact, "fluidflow.strength", fluidflow.outcome.Strength)
}

func (fluidflow *Fluidflow) LaminarReading() *FluidflowReading {
	return newFluidflowReading(fluidflow, func(outcome FluidflowOutcome) float64 {
		return outcome.LaminarScore
	})
}

func (fluidflow *Fluidflow) TurbulentReading() *FluidflowReading {
	return newFluidflowReading(fluidflow, func(outcome FluidflowOutcome) float64 {
		return outcome.TurbulentScore
	})
}

func (fluidflow *Fluidflow) InertialReading() *FluidflowReading {
	return newFluidflowReading(fluidflow, func(outcome FluidflowOutcome) float64 {
		return outcome.InertialScore
	})
}

func (fluidflow *Fluidflow) ViscousReading() *FluidflowReading {
	return newFluidflowReading(fluidflow, func(outcome FluidflowOutcome) float64 {
		return outcome.ViscousScore
	})
}

type FluidflowReading struct {
	artifact  *datura.Artifact
	fluidflow *Fluidflow
	project   func(FluidflowOutcome) float64
}

func newFluidflowReading(
	fluidflow *Fluidflow,
	project func(FluidflowOutcome) float64,
) *FluidflowReading {
	return &FluidflowReading{
		artifact:  datura.Acquire("fluidflow-reading", datura.Artifact_Type_json),
		fluidflow: fluidflow,
		project:   project,
	}
}

func (reading *FluidflowReading) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (reading *FluidflowReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.fluidflow != nil && reading.project != nil {
		value = reading.project(reading.fluidflow.outcome)
	}

	_ = reading.artifact.SetPayload(encodePayload(value))

	return reading.artifact.Read(p)
}

func (reading *FluidflowReading) Close() error {
	return nil
}
