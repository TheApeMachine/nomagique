package equation

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
)

/*
Fluidflow classifies laminar, turbulent, inertial, and viscous book-flow regimes.
The constructor artifact holds schema inputs; Write buffers inbound wire on its payload.
*/
type Fluidflow struct {
	artifact *datura.Artifact
}

/*
NewFluidflow returns a fluid-dynamics stage wired from config attributes.
*/
func NewFluidflow(artifact *datura.Artifact) io.ReadWriteCloser {
	if artifact == nil {
		artifact = datura.Acquire("fluidflow", datura.APPJSON)
	}

	if len(datura.Peek[[]string](artifact, "inputs")) == 0 {
		artifact.Poke(FluidflowInputKeys, "inputs")
	}

	return &Fluidflow{
		artifact: artifact,
	}
}

func (fluidflow *Fluidflow) Write(p []byte) (int, error) {
	fluidflow.artifact.WithPayload(p)
	return len(p), nil
}

func (fluidflow *Fluidflow) Read(p []byte) (int, error) {
	state, err := stageState(fluidflow.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	inputKeys := EnsureFeatureSchema(state, fluidflow.artifact, FluidflowInputKeys)

	fields, err := FeatureFields(state, inputKeys)

	if err != nil || len(fields) < len(FluidflowInputKeys) {
		return rejectStage(state, "equation: invalid stage input")
	}

	reynolds := fields[0]
	divergence := fields[1]
	viscosity := fields[2]
	midAddRate := fields[3]
	midExecuteRate := fields[4]
	laminarCeiling := fields[5]
	turbulentFloor := fields[6]
	turbulentReady := fields[7] > 0
	divergenceEdge := fields[8]
	icebergScore := fields[9]
	vorticity := fields[10]
	turbulence := fields[11]
	memory := fields[12]
	price := fields[13]
	spreadBPS := fields[14]
	changePct := fields[15]
	volume := fields[16]

	if price <= 0 || spreadBPS <= 0 || changePct <= 0 || volume <= 0 {
		return rejectStage(state, "equation: invalid stage input")
	}

	if viscosity <= 0 || math.IsNaN(reynolds) || math.IsInf(reynolds, 0) {
		return rejectStage(state, "equation: invalid stage input")
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

	if turbulence > 0 && turbulentReady {
		turbulentFromField := turbulence * reynolds

		if turbulentFromField > turbulentScore {
			turbulentScore = turbulentFromField
		}
	}

	if vorticity > 0 && turbulentReady {
		vortScore := vorticity * turbulence

		if vortScore > turbulentScore {
			turbulentScore = vortScore
		}
	}

	inertialScore := divergence
	viscousScore := 0.0

	if viscosity > 0 {
		viscousScore = divergence / viscosity
	}

	if icebergScore > 0 {
		viscousScore = math.Max(viscousScore, icebergScore)
	}

	if memory > 0 {
		viscousScore = math.Max(viscousScore, memory*viscosity)
	}

	midActivity := midAddRate + midExecuteRate

	if midActivity > 0 {
		executeShare := midExecuteRate / midActivity
		addShare := midAddRate / midActivity

		inertialScore = math.Max(inertialScore, executeShare*divergence)

		if addShare > executeShare {
			viscousScore = math.Max(viscousScore, addShare*viscosity)
		}

		if turbulentReady && executeShare > 0 {
			turbulentScore = math.Max(turbulentScore, executeShare*reynolds)
		}
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
		return rejectStage(state, "equation: invalid stage input")
	}

	return emitOutput(state, p, datura.Map[float64]{
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
	})
}

func (fluidflow *Fluidflow) Close() error {
	return nil
}
