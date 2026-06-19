package equation

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/statistic"
)

const (
	bookflowSnapshotHeader = 8
	bookflowHistoryHeader  = 3
	minBookGateHistory     = 3
)

/*
Bookflow classifies weighted book imbalance with touch skew and trade pressure.

Payload layout: weighted, level1, flat, flatOK, mid, spread, touchDepth,
tradePressure, weightedCount, level1Count, flatCount, then each abs history.
*/
type Bookflow struct {
	artifact *datura.Artifact
}

/*
NewBookflow returns a depth-flow stage.
*/
func NewBookflow() *Bookflow {
	return &Bookflow{
		artifact: datura.Acquire("bookflow", datura.APPJSON).RetainStageAttributes(),
	}
}

func (bookflow *Bookflow) StageArtifact() *datura.Artifact {
	return bookflow.artifact
}

func (bookflow *Bookflow) Write(p []byte) (int, error) {
	bootstrap := datura.Peek[datura.Map[float64]](bookflow.artifact, "output") == nil

	bookflow.artifact.Clear("sample")

	n, err := bookflow.artifact.Write(p)

	if bootstrap {
		bookflow.artifact.Clear("output")
	}

	return n, err
}

func (bookflow *Bookflow) Read(p []byte) (int, error) {
	batch := FloatBatch(bookflow.artifact)
	outcome := evaluateBookflow(batch)

	if !outcome.eligible || outcome.strength <= 0 {
		bookflow.artifact.Poke(datura.Map[float64]{"value": 0}, "output")

		return bookflow.artifact.Read(p)
	}

	bookflow.artifact.Poke(datura.Map[float64]{
		"value":        outcome.strength,
		"loadedScore":  outcome.loadedScore,
		"spoofScore":   outcome.spoofScore,
		"thinScore":    outcome.thinScore,
		"neutralScore": outcome.neutralScore,
		"category":     float64(outcome.category),
	}, "output")

	return bookflow.artifact.Read(p)
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

func evaluateBookflow(batch []float64) bookflowOutcome {
	headerEnd := bookflowSnapshotHeader + bookflowHistoryHeader

	if len(batch) < headerEnd {
		return bookflowOutcome{}
	}

	weighted := batch[0]
	level1 := batch[1]
	flat := batch[2]
	flatOK := batch[3] > 0
	mid := batch[4]
	spread := batch[5]
	touchDepth := batch[6]
	tradePressure := batch[7]

	weightedCount := int(batch[8])
	level1Count := int(batch[9])
	flatCount := int(batch[10])

	offset := headerEnd
	weightedHistory, offset, ok := bookflowSliceSegment(batch, offset, weightedCount)

	if !ok {
		return bookflowOutcome{}
	}

	level1History, offset, ok := bookflowSliceSegment(batch, offset, level1Count)

	if !ok {
		return bookflowOutcome{}
	}

	flatHistory, _, ok := bookflowSliceSegment(batch, offset, flatCount)

	if !ok {
		return bookflowOutcome{}
	}

	if mid <= 0 || spread <= 0 || len(weightedHistory) == 0 || len(level1History) == 0 {
		return bookflowOutcome{}
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

		pressureScale := bookflowLoadedPressureScale(tradePressure, weightedThreshold)

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

	if category == 0 || strength <= 0 {
		return bookflowOutcome{}
	}

	quoteVol := mid * touchDepth

	if quoteVol <= 0 {
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

func bookflowSliceSegment(batch []float64, offset, count int) ([]float64, int, bool) {
	if count < 0 || offset+count > len(batch) {
		return nil, offset, false
	}

	segment := append([]float64(nil), batch[offset:offset+count]...)

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

	return statistic.MedianOf(absoluteValues)
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

func bookflowLoadedPressureScale(tradePressure, weightedThreshold float64) float64 {
	if weightedThreshold <= 0 {
		return 1
	}

	confirmWeight := math.Abs(tradePressure) / (math.Abs(tradePressure) + weightedThreshold)

	scale := 1 + confirmWeight*tradePressure

	if scale < 0 {
		return 0
	}

	return scale
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
