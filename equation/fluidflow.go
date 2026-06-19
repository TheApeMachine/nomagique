package equation

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
)

const fluidflowPayloadFields = 14

/*
Fluidflow classifies laminar, turbulent, inertial, and viscous book-flow regimes.

Payload layout: reynolds, absDivergence, viscosity, midAddRate, midExecuteRate,
laminarCeiling, turbulentFloor, turbulentReady, divergenceEdge, icebergScore,
price, spreadBPS, changePct, volume.
*/
type Fluidflow struct {
	artifact *datura.Artifact
}

/*
NewFluidflow returns a fluid-dynamics stage for io.ReadWriter pipelines.
*/
func NewFluidflow() io.ReadWriteCloser {
	return &Fluidflow{
		artifact: datura.Acquire("fluidflow", datura.APPJSON),
	}
}

func (fluidflow *Fluidflow) Write(p []byte) (int, error) {
	return fluidflow.artifact.Write(p)
}

func (fluidflow *Fluidflow) Read(p []byte) (int, error) {
	batch := Features(fluidflow.artifact)

	if len(batch) < fluidflowPayloadFields {
		fluidflow.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return fluidflow.artifact.Read(p)
	}

	reynolds := batch[0]
	divergence := batch[1]
	viscosity := batch[2]
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
		fluidflow.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return fluidflow.artifact.Read(p)
	}

	if viscosity <= 0 || math.IsNaN(reynolds) || math.IsInf(reynolds, 0) {
		fluidflow.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return fluidflow.artifact.Read(p)
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
		fluidflow.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return fluidflow.artifact.Read(p)
	}

	fluidflow.artifact.Poke(datura.Map[float64]{
		"value":          strength,
		"laminarScore":   laminarScore,
		"turbulentScore": turbulentScore,
		"inertialScore":  inertialScore,
		"viscousScore":   viscousScore,
		"strength":       strength,
		"category":       float64(category),
		"price":          price,
		"spreadBPS":      spreadBPS,
		"changePct":      changePct,
		"volume":         volume,
	}, "output")

	return fluidflow.artifact.Read(p)
}

func (fluidflow *Fluidflow) Close() error {
	return nil
}
