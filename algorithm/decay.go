package algorithm

import (
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/probability"
	"gonum.org/v1/gonum/stat"
)

const decayPayloadHeader = 7

/*
DecayOutcome holds exit-decay scores from microstructure feature rings.
*/
type DecayOutcome struct {
	Mechanical float64
	Fragile    float64
	Thermal    float64
	Reversal   float64
	Urgency    float64
	Category   int
	Eligible   bool
}

/*
Decay classifies book thinning, spread widening, pressure fade, and imbalance flip.

Payload layout: lastPrice, bidDepthCount, askDepthCount, densityCount,
spreadCount, pressureCount, imbalanceCount, then each series oldest→newest.
*/
type Decay struct {
	artifact *datura.Artifact
	outcome  DecayOutcome
}

/*
NewDecay returns a microstructure decay stage for io.ReadWriter pipelines.
*/
func NewDecay() *Decay {
	return &Decay{
		artifact: datura.Acquire("decay", datura.Artifact_Type_json),
	}
}

func (decay *Decay) Write(p []byte) (int, error) {
	return decay.artifact.Write(p)
}

func (decay *Decay) Read(p []byte) (int, error) {
	rehydrateArtifact(&decay.artifact, "decay", datura.Artifact_Type_json)

	payload, err := decay.artifact.Payload()

	if err == nil {
		decay.outcome = decay.evaluate(payloadSamples(payload))
		decay.publishReadings()
	}

	return decay.artifact.Read(p)
}

func (decay *Decay) Close() error {
	return nil
}

/*
Outcome returns scores from the last Read.
*/
func (decay *Decay) Outcome() DecayOutcome {
	return decay.outcome
}

func (decay *Decay) evaluate(batch []float64) DecayOutcome {
	if len(batch) < decayPayloadHeader {
		return DecayOutcome{}
	}

	lastPrice := batch[0]

	if lastPrice <= 0 {
		return DecayOutcome{}
	}

	counts := batch[1:decayPayloadHeader]
	offset := decayPayloadHeader
	series := make([][]float64, len(counts))

	for index, count := range counts {
		segmentCount := int(count)

		if segmentCount < 0 || offset+segmentCount > len(batch) {
			return DecayOutcome{}
		}

		series[index] = batch[offset : offset+segmentCount]
		offset += segmentCount
	}

	longOutcome := decay.exitSide(series, 1)
	shortOutcome := decay.exitSide(series, -1)

	outcome := longOutcome

	if shortOutcome.Urgency > outcome.Urgency {
		outcome = shortOutcome
	}

	if outcome.Urgency <= 0 || outcome.Category == 0 {
		return DecayOutcome{}
	}

	outcome.Eligible = true

	return outcome
}

func (decay *Decay) exitSide(series [][]float64, side int) DecayOutcome {
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
		return DecayOutcome{}
	}

	urgency := 0.0

	for index, weight := range fusionWeights {
		urgency += weight * margins[index]
	}

	category := classifyDecay(mechanicalScore, widen, fade, flip)

	return DecayOutcome{
		Mechanical: margins[0],
		Fragile:    margins[1],
		Thermal:    margins[2],
		Reversal:   margins[3],
		Urgency:    urgency,
		Category:   category,
	}
}

func (decay *Decay) publishReadings() {
	pokeFloat(decay.artifact, "decay.mechanical", decay.outcome.Mechanical)
	pokeFloat(decay.artifact, "decay.fragile", decay.outcome.Fragile)
	pokeFloat(decay.artifact, "decay.thermal", decay.outcome.Thermal)
	pokeFloat(decay.artifact, "decay.reversal", decay.outcome.Reversal)
	pokeFloat(decay.artifact, "decay.urgency", decay.outcome.Urgency)
}

func (decay *Decay) MechanicalReading() *DecayReading {
	return newDecayReading(decay, func(outcome DecayOutcome) float64 {
		return outcome.Mechanical
	})
}

func (decay *Decay) FragileReading() *DecayReading {
	return newDecayReading(decay, func(outcome DecayOutcome) float64 {
		return outcome.Fragile
	})
}

func (decay *Decay) ThermalReading() *DecayReading {
	return newDecayReading(decay, func(outcome DecayOutcome) float64 {
		return outcome.Thermal
	})
}

func (decay *Decay) ReversalReading() *DecayReading {
	return newDecayReading(decay, func(outcome DecayOutcome) float64 {
		return outcome.Reversal
	})
}

type DecayReading struct {
	artifact *datura.Artifact
	decay    *Decay
	project  func(DecayOutcome) float64
}

func newDecayReading(
	decay *Decay,
	project func(DecayOutcome) float64,
) *DecayReading {
	return &DecayReading{
		artifact: datura.Acquire("decay-reading", datura.Artifact_Type_json),
		decay:    decay,
		project:  project,
	}
}

func (reading *DecayReading) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (reading *DecayReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.decay != nil && reading.project != nil {
		value = reading.project(reading.decay.outcome)
	}

	_ = reading.artifact.SetPayload(encodePayload(value))

	return reading.artifact.Read(p)
}

func (reading *DecayReading) Close() error {
	return nil
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
	priorPeak := pressures[0]

	for _, value := range pressures[:len(pressures)-1] {
		if value > priorPeak {
			priorPeak = value
		}
	}

	if side > 0 {
		if priorPeak <= 0 || recent >= priorPeak {
			return 0
		}

		return (priorPeak - recent) / math.Max(math.Abs(priorPeak), 1e-9)
	}

	if priorPeak >= 0 || recent <= priorPeak {
		return 0
	}

	return (recent - priorPeak) / math.Max(math.Abs(priorPeak), 1e-9)
}

func imbalanceFlip(imbalances []float64, side int) float64 {
	if len(imbalances) < 2 {
		return 0
	}

	recent := imbalances[len(imbalances)-1]
	prior := columnMean(imbalances[:len(imbalances)-1])

	if side > 0 && prior > 0 && recent < 0 {
		return math.Abs(recent) / math.Max(prior, 1e-9)
	}

	if side < 0 && prior < 0 && recent > 0 {
		return recent / math.Max(math.Abs(prior), 1e-9)
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
