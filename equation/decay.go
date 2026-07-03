package equation

import (
	"io"
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/probability"
	"gonum.org/v1/gonum/stat"
)

/*
Decay classifies book thinning, spread widening, pressure fade, and imbalance flip.
The constructor artifact holds schema inputs; Write buffers inbound wire on its payload.
*/
type Decay struct {
	artifact *datura.Artifact
}

/*
NewDecay returns a microstructure decay stage wired from config attributes.
*/
func NewDecay(artifact *datura.Artifact) io.ReadWriteCloser {
	return &Decay{
		artifact: artifact,
	}
}

func (decay *Decay) Write(p []byte) (int, error) {
	decay.artifact.WithPayload(p)
	return len(p), nil
}

func (decay *Decay) Read(p []byte) (int, error) {
	state, err := stageState(decay.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	inputKeys := EnsureFeatureSchema(state, decay.artifact, DecayInputKeys)

	fields, err := FeatureFields(state, inputKeys)

	if err != nil || len(fields) < len(DecayInputKeys) {
		return rejectStage(state, "equation: invalid stage input")
	}

	lastPrice := fields[0]

	if lastPrice <= 0 {
		return rejectStage(state, "equation: invalid stage input")
	}

	counts := fields[1:]
	offset := len(inputKeys)
	features := Features(state)
	series := make([][]float64, len(counts))

	for index, count := range counts {
		segmentCount := int(count)

		if segmentCount < 0 || offset+segmentCount > len(features) {
			return rejectStage(state, "equation: invalid stage input")
		}

		series[index] = features[offset : offset+segmentCount]
		offset += segmentCount
	}

	longOutcome, err := decay.exitSide(series, 1)

	if err != nil {
		state.Release()

		return 0, err
	}

	shortOutcome, err := decay.exitSide(series, -1)

	if err != nil {
		state.Release()

		return 0, err
	}

	mechanical := longOutcome.mechanical
	fragile := longOutcome.fragile
	thermal := longOutcome.thermal
	reversal := longOutcome.reversal
	urgency := longOutcome.urgency
	category := longOutcome.category

	if shortOutcome.urgency > urgency {
		mechanical = shortOutcome.mechanical
		fragile = shortOutcome.fragile
		thermal = shortOutcome.thermal
		reversal = shortOutcome.reversal
		urgency = shortOutcome.urgency
		category = shortOutcome.category
	}

	return emitOutput(state, p, datura.Map[float64]{
		"value":      urgency,
		"strength":   urgency,
		"mechanical": mechanical,
		"fragile":    fragile,
		"thermal":    thermal,
		"reversal":   reversal,
		"urgency":    urgency,
		"category":   float64(category),
	})
}

func (decay *Decay) Close() error {
	return nil
}

type decaySideOutcome struct {
	mechanical float64
	fragile    float64
	thermal    float64
	reversal   float64
	urgency    float64
	category   int
}

func (decay *Decay) exitSide(series [][]float64, side int) (decaySideOutcome, error) {
	bidDepths := series[0]
	askDepths := series[1]
	densities := series[2]
	spreads := series[3]
	pressures := series[4]
	imbalances := series[5]

	thinning := 0.0
	fade := 0.0
	flip := 0.0

	if side > 0 {
		thinning = depthTrend(bidDepths)
		fade = pressureFade(pressures, 1)
		flip = imbalanceFlip(imbalances, 1)
	}

	if side < 0 {
		thinning = depthTrend(askDepths)
		fade = pressureFade(pressures, -1)
		flip = imbalanceFlip(imbalances, -1)
	}

	widen := spreadWiden(spreads)
	collapse := depthTrend(densities)
	thinningScore := 0.0
	collapseScore := 0.0
	fragileScore := 0.0
	thermalScore := 0.0
	reversalScore := 0.0
	var err error

	if thinning > 0 {
		thinningScore, err = probability.MagnitudeMargin(thinning)

		if err != nil {
			return decaySideOutcome{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: thinning margin failed",
				err,
			))
		}
	}

	if collapse > 0 {
		collapseScore, err = probability.MagnitudeMargin(collapse)

		if err != nil {
			return decaySideOutcome{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: collapse margin failed",
				err,
			))
		}
	}

	if widen > 0 {
		fragileScore, err = probability.MagnitudeMargin(widen)

		if err != nil {
			return decaySideOutcome{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: spread margin failed",
				err,
			))
		}
	}

	if fade > 0 {
		thermalScore, err = probability.MagnitudeMargin(fade)

		if err != nil {
			return decaySideOutcome{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: pressure margin failed",
				err,
			))
		}
	}

	if flip > 0 {
		reversalScore, err = probability.MagnitudeMargin(flip)

		if err != nil {
			return decaySideOutcome{}, errnie.Error(errnie.Err(
				errnie.Validation,
				"equation decay: imbalance margin failed",
				err,
			))
		}
	}

	mechanicalScore := math.Max(thinningScore, collapseScore)

	margins := []float64{
		mechanicalScore,
		fragileScore,
		thermalScore,
		reversalScore,
	}

	fusionWeights, err := probability.SoftmaxScoresNormalized(margins)

	if err != nil {
		return decaySideOutcome{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"equation decay: softmax fusion failed",
			err,
		))
	}

	urgency := 0.0

	for index, weight := range fusionWeights {
		urgency += weight * margins[index]
	}

	category := classifyDecay(mechanicalScore, fragileScore, thermalScore, reversalScore)

	return decaySideOutcome{
		mechanical: margins[0],
		fragile:    margins[1],
		thermal:    margins[2],
		reversal:   margins[3],
		urgency:    urgency,
		category:   category,
	}, nil
}

func depthTrend(depths []float64) float64 {
	if len(depths) < 4 {
		return 0
	}

	splitIndex := len(depths) / 2

	if splitIndex < 1 {
		return 0
	}

	recent := columnMean(depths[splitIndex:])
	prior := columnMean(depths[:splitIndex])

	if prior <= 0 {
		return 0
	}

	return (prior - recent) / prior
}

func spreadWiden(spreads []float64) float64 {
	if len(spreads) < 4 {
		return 0
	}

	sorted := append([]float64(nil), spreads...)
	sort.Float64s(sorted)

	median := stat.Quantile(0.5, stat.LinInterp, sorted, nil)
	current := spreads[len(spreads)-1]

	if median <= 0 || current <= median {
		return 0
	}

	return (current - median) / median
}

func pressureFade(pressures []float64, side int) float64 {
	if len(pressures) < 3 {
		return 0
	}

	recent := pressures[len(pressures)-1]
	prior := pressures[0]

	for _, value := range pressures[:len(pressures)-1] {
		if side > 0 && value > prior {
			prior = value
		}

		if side < 0 && value < prior {
			prior = value
		}
	}

	if side > 0 {
		if prior <= 0 || recent >= prior {
			return 0
		}

		return (prior - recent) / math.Abs(prior)
	}

	if prior >= 0 || recent <= prior {
		return 0
	}

	return (recent - prior) / math.Abs(prior)
}

func imbalanceFlip(imbalances []float64, side int) float64 {
	if len(imbalances) < 2 {
		return 0
	}

	recent := imbalances[len(imbalances)-1]
	prior := columnMean(imbalances[:len(imbalances)-1])

	if side > 0 && prior > 0 && recent < 0 {
		return math.Abs(recent) / prior
	}

	if side < 0 && prior < 0 && recent > 0 {
		return recent / math.Abs(prior)
	}

	return 0
}

func classifyDecay(thinning, widen, fade, flip float64) int {
	best := thinning
	category := 1

	if widen > best {
		best = widen
		category = 2
	}

	if fade > best {
		best = fade
		category = 3
	}

	if flip > best {
		best = flip
		category = 4
	}

	if best <= 0 {
		return 0
	}

	return category
}

func columnMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0

	for _, value := range values {
		sum += value
	}

	return sum / float64(len(values))
}
