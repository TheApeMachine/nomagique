package equation

import (
	"math"

	"github.com/theapemachine/nomagique/statistic"
)

const minBookGateHistory = 3

/*
Bookflow classifies weighted book imbalance with touch skew and trade pressure.
*/
type Bookflow struct{}

/*
bookflowBaseline carries the empirical depth gates computed once per measure.
It keeps book-flow classification from re-copying and re-partitioning the same histories.
*/
type bookflowBaseline struct {
	weightedThreshold float64
	level1Threshold   float64
	flatThreshold     float64
	bookNotional      float64
	spoofContrast     float64
	spoofReady        bool
	thinningReady     bool
}

/*
BookflowInput contains the float-only book-flow inputs.
*/
type BookflowInput struct {
	Weighted            float64
	Level1              float64
	Flat                float64
	FlatOK              bool
	Mid                 float64
	Spread              float64
	TouchDepth          float64
	BookNotional        float64
	TradePressure       float64
	WeightedHistory     []float64
	Level1History       []float64
	FlatHistory         []float64
	BookNotionalHistory []float64
}

/*
BookflowOutput contains the float-only book-flow scores.
*/
type BookflowOutput struct {
	Value        float64
	Strength     float64
	LoadedScore  float64
	SpoofScore   float64
	ThinScore    float64
	NeutralScore float64
	Category     float64
	Ready        bool
}

/*
NewBookflow returns a depth-flow calculator.
*/
func NewBookflow() *Bookflow {
	return &Bookflow{}
}

/*
Measure calculates book-flow scores from floats without artifact transport.
*/
func (bookflow *Bookflow) Measure(input BookflowInput) (BookflowOutput, error) {
	if input.Mid <= 0 || input.Spread <= 0 {
		return BookflowOutput{}, nil
	}

	// Without prior book observations there is no empirical baseline against
	// which loaded, spoofed, thinning, or neutral depth can be distinguished.
	if len(input.WeightedHistory) == 0 || len(input.Level1History) == 0 {
		return BookflowOutput{}, nil
	}

	baseline := bookflowMeasureBaseline(input)

	spoofed := bookflowIsSpoofSkew(
		input.Weighted,
		input.Level1,
		baseline.weightedThreshold,
		baseline.level1Threshold,
		baseline.spoofContrast,
		baseline.spoofReady,
	)

	if input.FlatOK {
		spoofed = spoofed || bookflowIsSpoofSkew(
			input.Flat,
			input.Level1,
			baseline.weightedThreshold,
			baseline.level1Threshold,
			baseline.spoofContrast,
			baseline.spoofReady,
		)
	}

	thinning := bookflowIsBookThinning(
		input.BookNotional,
		baseline.bookNotional,
		baseline.thinningReady,
	)
	loadedThreshold := math.Max(baseline.weightedThreshold, baseline.level1Threshold)
	loaded := !spoofed && !thinning && input.Weighted*input.Level1 > 0 &&
		math.Abs(input.Weighted) > baseline.weightedThreshold &&
		math.Abs(input.Level1) > baseline.level1Threshold

	category := bookflowClassify(spoofed, thinning, loaded)

	loadedScore := 0.0

	if loaded {
		loadedScore = bookflowLoadedScore(
			input.Weighted,
			input.TradePressure,
			baseline.bookNotional,
			loadedThreshold,
		)
	}

	spoofScore := 0.0

	if spoofed {
		spoofScore = math.Abs(input.Weighted - input.Level1)
	}

	thinScore := 0.0

	if thinning {
		thinScore = (baseline.bookNotional - input.BookNotional) / baseline.bookNotional
	}

	neutralScore := 0.0

	if category == 4 {
		neutralScore = math.Max(0, 1-math.Abs(input.Weighted))
	}

	strength := math.Max(
		loadedScore,
		math.Max(spoofScore, math.Max(thinScore, neutralScore)),
	)

	quoteVol := input.Mid * input.TouchDepth

	if quoteVol <= 0 && strength > 0 {
		return BookflowOutput{}, nil
	}

	return BookflowOutput{
		Value:        strength,
		Strength:     strength,
		LoadedScore:  loadedScore,
		SpoofScore:   spoofScore,
		ThinScore:    thinScore,
		NeutralScore: neutralScore,
		Category:     float64(category),
		Ready:        true,
	}, nil
}

/*
bookflowMeasureBaseline computes every book-flow history statistic once.
It prevents the classifier from repeatedly copying the same slices for each gate.
*/
func bookflowMeasureBaseline(input BookflowInput) bookflowBaseline {
	baseline := bookflowBaseline{
		weightedThreshold: bookflowMedianAbsolute(input.WeightedHistory),
		level1Threshold:   bookflowMedianAbsolute(input.Level1History),
		spoofReady: len(input.WeightedHistory) >= minBookGateHistory &&
			len(input.Level1History) >= minBookGateHistory,
	}

	if len(input.FlatHistory) > 0 {
		baseline.flatThreshold = bookflowMedianAbsolute(input.FlatHistory)
	}

	denominator := baseline.weightedThreshold + baseline.level1Threshold

	if baseline.spoofReady && denominator > 0 {
		baseline.spoofContrast = baseline.weightedThreshold / denominator
	}

	if len(input.BookNotionalHistory) <= minBookGateHistory {
		return baseline
	}

	priorBookNotional := input.BookNotionalHistory[:len(input.BookNotionalHistory)-1]
	baseline.bookNotional, baseline.thinningReady = statistic.MedianOf(priorBookNotional)
	baseline.thinningReady = baseline.thinningReady && baseline.bookNotional > 0

	return baseline
}

/*
bookflowMedianAbsolute returns a robust central absolute-depth gate.
It uses statistic.MedianOf so caller-owned histories are never reordered.
*/
func bookflowMedianAbsolute(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	median, ok := statistic.MedianOf(values)

	if !ok {
		return 0
	}

	return median
}

func bookflowLoadedScore(
	weighted, tradePressure, bookNotional, weightedThreshold float64,
) float64 {
	loadedScore := math.Abs(weighted)

	if bookNotional <= 0 || weightedThreshold <= 0 || tradePressure == 0 {
		return loadedScore
	}

	pressureRatio := math.Abs(tradePressure) / bookNotional
	confirmation := pressureRatio / (pressureRatio + weightedThreshold)

	if weighted*tradePressure > 0 {
		return loadedScore + confirmation - loadedScore*confirmation
	}

	if weighted*tradePressure < 0 {
		return loadedScore * weightedThreshold / (weightedThreshold + pressureRatio)
	}

	return loadedScore
}

func bookflowIsSpoofSkew(
	weighted, level1, weightedThreshold, level1Threshold, spoofContrast float64,
	ready bool,
) bool {
	if !ready {
		return false
	}

	if math.Abs(weighted) < weightedThreshold {
		return false
	}

	if weighted*level1 >= 0 {
		return false
	}

	return math.Abs(level1) >= level1Threshold*spoofContrast
}

func bookflowIsBookThinning(
	bookNotional, baselineNotional float64,
	ready bool,
) bool {
	if !ready || bookNotional <= 0 || baselineNotional <= 0 {
		return false
	}

	return bookNotional < baselineNotional
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
