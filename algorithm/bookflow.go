package algorithm

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
BookflowOutcome holds book-imbalance classification scores.
*/
type BookflowOutcome struct {
	LoadedScore  float64
	SpoofScore   float64
	ThinScore    float64
	NeutralScore float64
	Strength     float64
	Category     int
	Eligible     bool
	Mid          float64
	Spread       float64
	TouchDepth   float64
	QuoteVolume  float64
	Elapsed      float64
}

/*
Bookflow classifies weighted book imbalance with touch skew and trade pressure.

Payload layout: weighted, level1, flat, flatOK, mid, spread, touchDepth,
tradePressure, weightedCount, level1Count, flatCount, then each abs history.
*/
type Bookflow struct {
	artifact *datura.Artifact
	outcome  BookflowOutcome
}

/*
NewBookflow returns a depth-flow stage for io.ReadWriter pipelines.
*/
func NewBookflow() *Bookflow {
	return &Bookflow{
		artifact: datura.Acquire("bookflow", datura.Artifact_Type_json),
	}
}

func (bookflow *Bookflow) Write(p []byte) (int, error) {
	return bookflow.artifact.Write(p)
}

func (bookflow *Bookflow) Read(p []byte) (int, error) {
	rehydrateArtifact(&bookflow.artifact, "bookflow", datura.Artifact_Type_json)

	payload, err := bookflow.artifact.Payload()

	if err == nil {
		bookflow.outcome = bookflow.evaluate(payloadSamples(payload))
		bookflow.publishReadings()
	}

	return bookflow.artifact.Read(p)
}

func (bookflow *Bookflow) Close() error {
	return nil
}

/*
Outcome returns scores from the last Read.
*/
func (bookflow *Bookflow) Outcome() BookflowOutcome {
	return bookflow.outcome
}

func (bookflow *Bookflow) evaluate(batch []float64) BookflowOutcome {
	headerEnd := bookflowSnapshotHeader + bookflowHistoryHeader

	if len(batch) < headerEnd {
		return BookflowOutcome{}
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
	weightedHistory, offset, ok := sliceSegment(batch, offset, weightedCount)

	if !ok {
		return BookflowOutcome{}
	}

	level1History, offset, ok := sliceSegment(batch, offset, level1Count)

	if !ok {
		return BookflowOutcome{}
	}

	flatHistory, _, ok := sliceSegment(batch, offset, flatCount)

	if !ok {
		return BookflowOutcome{}
	}

	if mid <= 0 || spread <= 0 || len(weightedHistory) == 0 || len(level1History) == 0 {
		return BookflowOutcome{}
	}

	weightedThreshold := medianAbsolute(weightedHistory)
	level1Threshold := medianAbsolute(level1History)
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
		neutralScore = 1 - math.Abs(weighted)
	}

	strength := math.Abs(weighted)

	if category == 2 {
		strength = spoofScore
	}

	if category == 3 {
		strength = math.Abs(thinScore)
	}

	if category == 0 || strength <= 0 {
		return BookflowOutcome{}
	}

	quoteVol := mid * touchDepth

	if quoteVol <= 0 {
		return BookflowOutcome{}
	}

	return BookflowOutcome{
		LoadedScore:  loadedScore,
		SpoofScore:   spoofScore,
		ThinScore:    thinScore,
		NeutralScore: neutralScore,
		Strength:     strength,
		Category:     category,
		Eligible:     true,
		Mid:          mid,
		Spread:       spread,
		TouchDepth:   touchDepth,
		QuoteVolume:  quoteVol,
	}
}

func sliceSegment(batch []float64, offset, count int) ([]float64, int, bool) {
	if count < 0 || offset+count > len(batch) {
		return nil, offset, false
	}

	segment := append([]float64(nil), batch[offset:offset+count]...)

	return segment, offset + count, true
}

func medianAbsolute(values []float64) float64 {
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

	weightedMedian := medianAbsolute(weightedHistory)
	level1Median := medianAbsolute(level1History)
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

	weightedMedian := medianAbsolute(weightedHistory)
	flatMedian := medianAbsolute(flatHistory)

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

	return 1 + confirmWeight*tradePressure
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

func (bookflow *Bookflow) publishReadings() {
	pokeFloat(bookflow.artifact, "bookflow.loaded", bookflow.outcome.LoadedScore)
	pokeFloat(bookflow.artifact, "bookflow.spoof", bookflow.outcome.SpoofScore)
	pokeFloat(bookflow.artifact, "bookflow.thin", bookflow.outcome.ThinScore)
	pokeFloat(bookflow.artifact, "bookflow.neutral", bookflow.outcome.NeutralScore)
	pokeFloat(bookflow.artifact, "bookflow.strength", bookflow.outcome.Strength)
}

func (bookflow *Bookflow) LoadedReading() *BookflowReading {
	return newBookflowReading(bookflow, func(outcome BookflowOutcome) float64 {
		return outcome.LoadedScore
	})
}

func (bookflow *Bookflow) SpoofReading() *BookflowReading {
	return newBookflowReading(bookflow, func(outcome BookflowOutcome) float64 {
		return outcome.SpoofScore
	})
}

func (bookflow *Bookflow) ThinningReading() *BookflowReading {
	return newBookflowReading(bookflow, func(outcome BookflowOutcome) float64 {
		return outcome.ThinScore
	})
}

func (bookflow *Bookflow) NeutralReading() *BookflowReading {
	return newBookflowReading(bookflow, func(outcome BookflowOutcome) float64 {
		return outcome.NeutralScore
	})
}

type BookflowReading struct {
	artifact *datura.Artifact
	bookflow *Bookflow
	project  func(BookflowOutcome) float64
}

func newBookflowReading(
	bookflow *Bookflow,
	project func(BookflowOutcome) float64,
) *BookflowReading {
	return &BookflowReading{
		artifact: datura.Acquire("bookflow-reading", datura.Artifact_Type_json),
		bookflow: bookflow,
		project:  project,
	}
}

func (reading *BookflowReading) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (reading *BookflowReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.bookflow != nil && reading.project != nil {
		value = reading.project(reading.bookflow.outcome)
	}

	_ = reading.artifact.SetPayload(encodePayload(value))

	return reading.artifact.Read(p)
}

func (reading *BookflowReading) Close() error {
	return nil
}
