package market

import "github.com/shopspring/decimal"

/*
BookSide is one half of the book (bids or asks).
*/
type BookSide struct {
	Direction SideKind          `json:"direction,omitempty"`
	High      *Level            `json:"high,omitempty"`
	Low       *Level            `json:"low,omitempty"`
	Last      *Level            `json:"last,omitempty"`
	Levels    map[string]*Level `json:"levels,omitempty"`
}

func NewBookSide() *BookSide {
	return &BookSide{
		Levels: make(map[string]*Level),
	}
}

func (side *BookSide) FindAdjacent(price *decimal.Decimal) *Level {
	if side.High == nil || side.Low == nil {
		return nil
	}

	if price.Cmp(*side.High.Price) > 0 {
		return side.High
	}

	if price.Cmp(*side.Low.Price) < 0 {
		return side.Low
	}

	highDistance := side.High.Price.Sub(*price)
	lowDistance := price.Sub(*side.Low.Price)

	if highDistance.Cmp(lowDistance) > 0 {
		return side.FindAdjacentBelow(price)
	}

	return side.FindAdjacentAbove(price)
}

func (side *BookSide) FindAdjacentBelow(price *decimal.Decimal) *Level {
	if side.Low == nil || price.Cmp(*side.Low.Price) <= 0 {
		return nil
	}

	nearest := side.Low

	for {
		next := nearest.Higher

		if next == nil {
			break
		}

		nearestDiff := nearest.Price.Sub(*price).Abs()
		nextDiff := next.Price.Sub(*price).Abs()

		if nearestDiff.Cmp(nextDiff) < 0 || next.Price.Cmp(*price) >= 0 {
			break
		}

		nearest = next
	}

	return nearest
}

func (side *BookSide) FindAdjacentAbove(price *decimal.Decimal) *Level {
	if side.High == nil || price.Cmp(*side.High.Price) >= 0 {
		return nil
	}

	nearest := side.High

	for {
		next := nearest.Lower

		if next == nil {
			break
		}

		nearestDiff := nearest.Price.Sub(*price).Abs()
		nextDiff := next.Price.Sub(*price).Abs()

		if nearestDiff.Cmp(nextDiff) < 0 || next.Price.Cmp(*price) <= 0 {
			break
		}

		nearest = next
	}

	return nearest
}

func (side *BookSide) apply(delta *Delta) {
	level, ok := side.Levels[delta.Price.String()]

	if !ok && delta.Quantity.Sign() > 0 {
		side.insert(delta)
		return
	}

	if ok {
		level.update(delta)
	}

	if level != nil && level.Quantity.Sign() <= 0 {
		side.delete(level)
	}
}

func (side *BookSide) insert(delta *Delta) {
	level := NewLevel()
	level.update(delta)
	nearest := side.FindAdjacent(level.Price)

	if nearest == nil || side.High == nil || level.Price.Cmp(*side.High.Price) > 0 {
		side.High = level
	}

	if nearest == nil || side.Low == nil || level.Price.Cmp(*side.Low.Price) < 0 {
		side.Low = level
	}

	if nearest != nil {
		if level.Price.Cmp(*nearest.Price) > 0 {
			level.Lower = nearest
			level.Higher = nearest.Higher
			nearest.Higher = level

			if level.Higher != nil {
				level.Higher.Lower = level
			}
		} else if level.Price.Cmp(*nearest.Price) < 0 {
			level.Higher = nearest
			level.Lower = nearest.Lower
			nearest.Lower = level

			if level.Lower != nil {
				level.Lower.Higher = level
			}
		}
	}

	side.Levels[level.Price.String()] = level
}

func (side *BookSide) delete(level *Level) {
	if side.High == level {
		side.High = level.Lower
	}

	if side.Low == level {
		side.Low = level.Higher
	}

	if level.Lower != nil {
		level.Lower.Higher = level.Higher
	}

	if level.Higher != nil {
		level.Higher.Lower = level.Lower
	}

	delete(side.Levels, level.Price.String())
}
