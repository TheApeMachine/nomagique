package equation

import (
	"io"
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/probability"
	"gonum.org/v1/gonum/stat"
)

const decayPayloadHeader = 7

/*
Decay classifies book thinning, spread widening, pressure fade, and imbalance flip.

Payload layout: lastPrice, bidDepthCount, askDepthCount, densityCount,
spreadCount, pressureCount, imbalanceCount, then each series oldest→newest.
*/
type Decay struct {
	artifact *datura.Artifact
}

/*
NewDecay returns a microstructure decay stage for io.ReadWriter pipelines.
*/
func NewDecay() io.ReadWriteCloser {
	return &Decay{
		artifact: datura.Acquire("decay", datura.APPJSON),
	}
}

func (decay *Decay) Write(p []byte) (int, error) {
	return decay.artifact.Write(p)
}

func (decay *Decay) Read(p []byte) (int, error) {
	batch := Features(decay.artifact)

	if len(batch) < decayPayloadHeader {
		decay.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return decay.artifact.Read(p)
	}

	lastPrice := batch[0]

	if lastPrice <= 0 {
		decay.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return decay.artifact.Read(p)
	}

	counts := batch[1:decayPayloadHeader]
	offset := decayPayloadHeader
	series := make([][]float64, len(counts))

	for index, count := range counts {
		segmentCount := int(count)

		if segmentCount < 0 || offset+segmentCount > len(batch) {
			decay.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

			return decay.artifact.Read(p)
		}

		series[index] = batch[offset : offset+segmentCount]
		offset += segmentCount
	}

	longOutcome := decay.exitSide(series, 1)
	shortOutcome := decay.exitSide(series, -1)

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

	if urgency <= 0 || category == 0 {
		decay.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return decay.artifact.Read(p)
	}

	decay.artifact.Poke(datura.Map[float64]{
		"value":      urgency,
		"mechanical": mechanical,
		"fragile":    fragile,
		"thermal":    thermal,
		"reversal":   reversal,
		"urgency":    urgency,
		"category":   float64(category),
	}, "output")

	return decay.artifact.Read(p)
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

func (decay *Decay) exitSide(series [][]float64, side int) decaySideOutcome {
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
	collapseMargin := componentMargin(collapse)
	mechanicalScore := math.Max(componentMargin(thinning), collapseMargin)

	margins := []float64{
		mechanicalScore,
		componentMargin(widen),
		componentMargin(fade),
		componentMargin(flip),
		collapseMargin,
	}

	fusionWeights, fusionErr := probability.SoftmaxScoresNormalized(margins)

	if fusionErr != nil {
		return decaySideOutcome{}
	}

	urgency := 0.0

	for index, weight := range fusionWeights {
		urgency += weight * margins[index]
	}

	category := classifyDecay(mechanicalScore, widen, fade, flip)

	return decaySideOutcome{
		mechanical: margins[0],
		fragile:    margins[1],
		thermal:    margins[2],
		reversal:   margins[3],
		urgency:    urgency,
		category:   category,
	}
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

func componentMargin(value float64) float64 {
	if value <= 0 {
		return 0
	}

	return value / (1 + value)
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
