package equation

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/statistic"
)

const minBookGateHistory = 3

/*
Bookflow classifies weighted book imbalance with touch skew and trade pressure.
The constructor artifact holds schema inputs; Write buffers inbound wire on its payload.
*/
type Bookflow struct {
	artifact *datura.Artifact
}

/*
NewBookflow returns a depth-flow stage wired from config attributes.
*/
func NewBookflow(artifact *datura.Artifact) *Bookflow {
	return &Bookflow{
		artifact: artifact,
	}
}

func (bookflow *Bookflow) Read(p []byte) (int, error) {
	state, err := stageState(bookflow.artifact.DecryptPayload())

	if err != nil {
		return 0, err
	}

	inputKeys := EnsureFeatureSchema(state, bookflow.artifact, BookflowInputKeys)
	outcome := evaluateBookflow(state, inputKeys)

	if !outcome.eligible {
		state.Release()

		return 0, io.EOF
	}

	return emitOutput(state, p, datura.Map[float64]{
		"value":        outcome.strength,
		"strength":     outcome.strength,
		"loadedScore":  outcome.loadedScore,
		"spoofScore":   outcome.spoofScore,
		"thinScore":    outcome.thinScore,
		"neutralScore": outcome.neutralScore,
		"category":     float64(outcome.category),
	})
}

func (bookflow *Bookflow) Write(p []byte) (int, error) {
	bookflow.artifact.WithPayload(p)
	return len(p), nil
}

func (bookflow *Bookflow) Close() error {
	return nil
}

type bookflowOutcome struct {
	loadedScore  float64
	spoofScore   float64
	thinScore    float64
	neutralScore float64
	strength     float64
	category     int
	eligible     bool
}

func evaluateBookflow(state *datura.Artifact, inputKeys []string) bookflowOutcome {
	fields, err := FeatureFields(state, inputKeys)

	if err != nil || len(fields) < len(BookflowInputKeys) {
		return bookflowOutcome{}
	}

	weighted := fields[0]
	level1 := fields[1]
	flat := fields[2]
	flatOK := fields[3] > 0
	mid := fields[4]
	spread := fields[5]
	touchDepth := fields[6]
	tradePressure := fields[7]
	weightedCount := int(fields[8])
	level1Count := int(fields[9])
	flatCount := int(fields[10])

	offset := len(inputKeys)
	features := Features(state)

	weightedHistory, offset, ok := bookflowHistorySegment(features, offset, weightedCount)

	if !ok {
		return bookflowOutcome{}
	}

	level1History, offset, ok := bookflowHistorySegment(features, offset, level1Count)

	if !ok {
		return bookflowOutcome{}
	}

	flatHistory, _, ok := bookflowHistorySegment(features, offset, flatCount)

	if !ok {
		return bookflowOutcome{}
	}

	if mid <= 0 || spread <= 0 || len(weightedHistory) == 0 || len(level1History) == 0 {
		return bookflowOutcome{
			eligible: true,
		}
	}

	weightedThreshold := bookflowMedianAbsolute(weightedHistory)
	level1Threshold := bookflowMedianAbsolute(level1History)
	spoofContrast := bookflowSpoofContrast(weightedHistory, level1History)
	depthGate := bookflowThinningGate(weightedHistory, flatHistory)

	spoofed := bookflowIsSpoofSkew(
		weighted, level1, weightedThreshold, level1Threshold, spoofContrast,
	)

	if flatOK {
		spoofed = spoofed || bookflowIsSpoofSkew(
			flat, level1, weightedThreshold, level1Threshold, spoofContrast,
		)
	}

	thinning := bookflowIsBookThinning(weighted, flat, flatOK, depthGate)
	loaded := !spoofed && !thinning &&
		math.Abs(weighted) >= weightedThreshold &&
		weightedThreshold > 0

	category := bookflowClassify(spoofed, thinning, loaded)

	loadedScore := 0.0

	if loaded {
		loadedScore = math.Abs(weighted)

		pressureScale := bookflowLoadedPressureScale(weighted, tradePressure, weightedThreshold)

		if pressureScale > 0 {
			loadedScore *= pressureScale
		}
	}

	spoofScore := 0.0

	if spoofed {
		spoofScore = math.Abs(weighted - level1)
	}

	thinScore := 0.0

	if thinning {
		thinScore = math.Abs(weighted) - math.Abs(flat)
	}

	neutralScore := 0.0

	if category == 4 {
		neutralScore = math.Max(0, 1-math.Abs(weighted))
	}

	strength := math.Abs(weighted)

	if category == 2 {
		strength = spoofScore
	}

	if category == 3 {
		strength = math.Abs(thinScore)
	}

	if category == 4 {
		strength = neutralScore
	}

	quoteVol := mid * touchDepth

	if quoteVol <= 0 && strength > 0 {
		return bookflowOutcome{}
	}

	return bookflowOutcome{
		loadedScore:  loadedScore,
		spoofScore:   spoofScore,
		thinScore:    thinScore,
		neutralScore: neutralScore,
		strength:     strength,
		category:     category,
		eligible:     true,
	}
}

func bookflowHistorySegment(features []float64, offset, count int) ([]float64, int, bool) {
	if count < 0 || offset+count > len(features) {
		return nil, offset, false
	}

	segment := append([]float64(nil), features[offset:offset+count]...)

	return segment, offset + count, true
}

func bookflowMedianAbsolute(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	absoluteValues := make([]float64, len(values))

	for index, value := range values {
		absoluteValues[index] = math.Abs(value)
	}

	median, ok := statistic.MedianOf(absoluteValues)

	if !ok {
		return 0
	}

	return median
}

func bookflowSpoofContrast(weightedHistory, level1History []float64) float64 {
	if len(weightedHistory) < minBookGateHistory || len(level1History) < minBookGateHistory {
		return 0
	}

	weightedMedian := bookflowMedianAbsolute(weightedHistory)
	level1Median := bookflowMedianAbsolute(level1History)
	denominator := weightedMedian + level1Median

	if denominator <= 0 {
		return 0
	}

	return weightedMedian / denominator
}

func bookflowThinningGate(weightedHistory, flatHistory []float64) float64 {
	if len(weightedHistory) < minBookGateHistory || len(flatHistory) < minBookGateHistory {
		return 0
	}

	weightedMedian := bookflowMedianAbsolute(weightedHistory)
	flatMedian := bookflowMedianAbsolute(flatHistory)

	if weightedMedian <= 0 {
		return 0
	}

	return flatMedian / weightedMedian
}

func bookflowLoadedPressureScale(weighted, tradePressure, weightedThreshold float64) float64 {
	if weightedThreshold <= 0 {
		return 1
	}

	confirmWeight := math.Abs(tradePressure) / (math.Abs(tradePressure) + weightedThreshold)
	if weighted*tradePressure > 0 {
		return 1 + confirmWeight
	}

	if weighted*tradePressure < 0 {
		return 1 - confirmWeight
	}

	return 1
}

func bookflowIsSpoofSkew(
	weighted, level1, weightedThreshold, level1Threshold, spoofContrast float64,
) bool {
	if math.Abs(weighted) < weightedThreshold {
		return false
	}

	if weighted*level1 >= 0 {
		return false
	}

	if spoofContrast <= 0 {
		return false
	}

	return math.Abs(level1) >= level1Threshold*spoofContrast
}

func bookflowIsBookThinning(
	weighted, flat float64,
	flatOK bool,
	depthGate float64,
) bool {
	if !flatOK || math.Abs(weighted) <= 0 {
		return false
	}

	if depthGate <= 0 {
		return false
	}

	return math.Abs(flat) < depthGate*math.Abs(weighted)
}

func bookflowClassify(spoofed, thinning, loaded bool) int {
	if spoofed {
		return 2
	}

	if thinning {
		return 3
	}

	if loaded {
		return 1
	}

	return 4
}
