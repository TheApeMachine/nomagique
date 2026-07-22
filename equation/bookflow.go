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
BookflowInput contains the float-only book-flow inputs.
*/
type BookflowInput struct {
	Weighted        float64
	Level1          float64
	Flat            float64
	FlatOK          bool
	Mid             float64
	Spread          float64
	TouchDepth      float64
	TradePressure   float64
	WeightedHistory []float64
	Level1History   []float64
	FlatHistory     []float64
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

	weightedThreshold := bookflowMedianAbsolute(input.WeightedHistory)
	level1Threshold := bookflowMedianAbsolute(input.Level1History)
	spoofContrast := bookflowSpoofContrast(input.WeightedHistory, input.Level1History)
	spoofReady := len(input.WeightedHistory) >= minBookGateHistory &&
		len(input.Level1History) >= minBookGateHistory
	depthGate, balancedDepthHistory := bookflowThinningGate(
		input.WeightedHistory,
		input.FlatHistory,
	)
	thinningReady := len(input.WeightedHistory) >= minBookGateHistory &&
		len(input.FlatHistory) >= minBookGateHistory

	spoofed := bookflowIsSpoofSkew(
		input.Weighted, input.Level1, weightedThreshold, level1Threshold,
		spoofContrast, spoofReady,
	)

	if input.FlatOK {
		spoofed = spoofed || bookflowIsSpoofSkew(
			input.Flat, input.Level1, weightedThreshold, level1Threshold,
			spoofContrast, spoofReady,
		)
	}

	thinning := bookflowIsBookThinning(
		input.Weighted,
		input.Flat,
		input.FlatOK,
		depthGate,
		thinningReady,
		balancedDepthHistory,
	)
	loadedThreshold := math.Max(weightedThreshold, level1Threshold)
	loaded := !spoofed && !thinning && input.Weighted*input.Level1 > 0 &&
		math.Abs(input.Weighted) > weightedThreshold &&
		math.Abs(input.Level1) > level1Threshold

	category := bookflowClassify(spoofed, thinning, loaded)

	loadedScore := 0.0

	if loaded {
		loadedScore = math.Abs(input.Weighted)

		pressureScale := bookflowLoadedPressureScale(
			input.Weighted,
			input.TradePressure,
			loadedThreshold,
		)

		if pressureScale > 0 {
			loadedScore *= pressureScale
		}
	}

	spoofScore := 0.0

	if spoofed {
		spoofScore = math.Abs(input.Weighted - input.Level1)
	}

	thinScore := 0.0

	if thinning {
		thinScore = depthGate*math.Abs(input.Weighted) - math.Abs(input.Flat)

		if balancedDepthHistory {
			thinScore = math.Abs(input.Weighted) - math.Abs(input.Flat)
		}
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

func bookflowThinningGate(
	weightedHistory,
	flatHistory []float64,
) (float64, bool) {
	if len(weightedHistory) < minBookGateHistory || len(flatHistory) < minBookGateHistory {
		return 0, false
	}

	weightedMedian := bookflowMedianAbsolute(weightedHistory)
	flatMedian := bookflowMedianAbsolute(flatHistory)

	if weightedMedian <= 0 {
		return 0, flatMedian <= 0
	}

	return flatMedian / weightedMedian, false
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
	weighted, flat float64,
	flatOK bool,
	depthGate float64,
	ready bool,
	balancedHistory bool,
) bool {
	if !ready || !flatOK || math.Abs(weighted) <= 0 {
		return false
	}

	if balancedHistory {
		return math.Abs(flat) < math.Abs(weighted)
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
