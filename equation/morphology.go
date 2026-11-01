package equation

import (
	"math"
	"sort"

	"github.com/theapemachine/errnie"
)

/*
Morphology classifies displayed book shape: stability, ask vacuum, and
ask-heavy notional lean.
*/
type Morphology struct{}

/*
MorphologyLevel is one price/quantity level.
*/
type MorphologyLevel struct {
	Price    float64
	Quantity float64
}

/*
MorphologyPoint is one normalized book-curve coordinate.
*/
type MorphologyPoint struct {
	Pos    float64
	Weight float64
}

/*
MorphologyInput carries both sides of the book plus sample-owned continuity.
*/
type MorphologyInput struct {
	Bids        []MorphologyLevel
	Asks        []MorphologyLevel
	AskVacuum   float64
	MorphChange float64
	HasMorph    bool
}

/*
MorphologyOutput contains book-shape scores.
*/
type MorphologyOutput struct {
	Value           float64 `json:"value"`
	ShapeStable     float64 `json:"shape_stable"`
	AskVacuum       float64 `json:"ask_vacuum"`
	BidNotionalLean float64 `json:"bid_notional_lean"`
	Strength        float64 `json:"strength"`
}

/*
NewMorphology returns a book-shape morphology calculator.
*/
func NewMorphology() *Morphology {
	return &Morphology{}
}

/*
Map returns scored labels for the grid.
*/
func (output *MorphologyOutput) Map() map[string]float64 {
	if output == nil {
		return nil
	}

	return map[string]float64{
		"shape_stable":      output.ShapeStable,
		"ask_vacuum":        output.AskVacuum,
		"bid_notional_lean": output.BidNotionalLean,
		"value":             output.Value,
		"strength":          output.Strength,
	}
}

/*
Measure scores book morphology from float-only levels.
*/
func (morphology *Morphology) Measure(
	input MorphologyInput,
) (MorphologyOutput, error) {
	if len(input.Bids) == 0 || len(input.Asks) == 0 {
		return MorphologyOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"morphology: both book sides required",
			nil,
		))
	}

	bestBid := input.Bids[0].Price
	bestAsk := input.Asks[0].Price

	for _, level := range input.Bids {
		if level.Price > bestBid {
			bestBid = level.Price
		}
	}

	for _, level := range input.Asks {
		if level.Price < bestAsk {
			bestAsk = level.Price
		}
	}

	if bestAsk <= bestBid || bestBid <= 0 {
		return MorphologyOutput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"morphology: book is crossed or non-positive",
			nil,
		))
	}

	lean := bidNotionalLean(input.Bids, input.Asks)
	stable := 0.0

	if input.HasMorph {
		stable = 1.0
		if input.MorphChange > 0 {
			stable = 1 / (1 + input.MorphChange)
		}
	}

	vacuum := math.Max(0, input.AskVacuum)
	strength := math.Max(stable, math.Max(vacuum, lean))

	return MorphologyOutput{
		Value:           strength,
		ShapeStable:     stable,
		AskVacuum:       vacuum,
		BidNotionalLean: lean,
		Strength:        strength,
	}, nil
}

func bidNotionalLean(bids, asks []MorphologyLevel) float64 {
	bidNotional := 0.0
	for _, level := range bids {
		if level.Price > 0 && level.Quantity > 0 {
			bidNotional += level.Price * level.Quantity
		}
	}

	askNotional := 0.0
	for _, level := range asks {
		if level.Price > 0 && level.Quantity > 0 {
			askNotional += level.Price * level.Quantity
		}
	}

	total := bidNotional + askNotional
	if total <= 0 {
		return 0
	}

	return math.Max(0, (askNotional-bidNotional)/total)
}

/*
MorphBookCurve builds the normalized combined bid/ask curve used for morph change.
*/
func MorphBookCurve(
	bids, asks []MorphologyLevel,
	mid, spread float64,
) ([]MorphologyPoint, error) {
	if spread <= 0 || mid <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"morphology: positive mid and spread required",
			nil,
		))
	}

	bidPoints, err := morphSideCurve(bids, mid, spread)
	if err != nil {
		return nil, err
	}

	askPoints, err := morphSideCurve(asks, mid, spread)
	if err != nil {
		return nil, err
	}

	curve := make([]MorphologyPoint, 0, len(bidPoints)+len(askPoints))
	for _, point := range bidPoints {
		curve = append(curve, MorphologyPoint{Pos: point.Pos, Weight: point.Weight * 0.5})
	}
	for _, point := range askPoints {
		curve = append(curve, MorphologyPoint{Pos: point.Pos, Weight: point.Weight * 0.5})
	}

	return curve, nil
}

/*
MorphWalk returns KS distance and path distance between two normalized curves.
*/
func MorphWalk(streamA, streamB []MorphologyPoint) (float64, float64) {
	if len(streamA) == 0 || len(streamB) == 0 {
		return 0, 0
	}

	var totalA, totalB float64
	for _, point := range streamA {
		totalA += point.Weight
	}
	for _, point := range streamB {
		totalB += point.Weight
	}
	if totalA == 0 || totalB == 0 {
		return 0, 0
	}

	var idxA, idxB int
	var cumA, cumB float64
	var prevPos, ks, distance float64
	first := true

	for idxA < len(streamA) || idxB < len(streamB) {
		pos := 0.0
		if idxA < len(streamA) && idxB < len(streamB) {
			pos = streamA[idxA].Pos
			if streamB[idxB].Pos < pos {
				pos = streamB[idxB].Pos
			}
		}
		if idxA < len(streamA) && idxB >= len(streamB) {
			pos = streamA[idxA].Pos
		}
		if idxB < len(streamB) && idxA >= len(streamA) {
			pos = streamB[idxB].Pos
		}

		if !first {
			distance += math.Abs(cumA-cumB) * (pos - prevPos)
		}
		first = false

		for idxA < len(streamA) && streamA[idxA].Pos == pos {
			cumA += streamA[idxA].Weight / totalA
			idxA++
		}
		for idxB < len(streamB) && streamB[idxB].Pos == pos {
			cumB += streamB[idxB].Weight / totalB
			idxB++
		}

		diff := math.Abs(cumA - cumB)
		if diff > ks {
			ks = diff
		}
		prevPos = pos
	}

	return ks, distance
}

func morphSideCurve(
	levels []MorphologyLevel,
	mid, spread float64,
) ([]MorphologyPoint, error) {
	points := make([]MorphologyPoint, 0, len(levels))
	total := 0.0

	for _, level := range levels {
		if level.Price <= 0 || level.Quantity < 0 {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"morphology: non-positive level",
				nil,
			))
		}

		notional := level.Price * level.Quantity
		total += notional
		points = append(points, MorphologyPoint{
			Pos:    (level.Price - mid) / spread,
			Weight: notional,
		})
	}

	if total <= 0 {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"morphology: empty notional curve",
			nil,
		))
	}

	for index := range points {
		points[index].Weight /= total
	}

	sort.SliceStable(points, func(left, right int) bool {
		return points[left].Pos < points[right].Pos
	})

	return points, nil
}
