package flow

import (
	"math"
	"math/big"
	"sort"

	"github.com/theapemachine/errnie"
)

type levelOp struct {
	tick     int64
	quantity float64
	previous float64
	touch    bool
}

func (sideBook *SideBook) prepare(levels []BookLevel) ([]levelOp, error) {
	ops := make([]levelOp, 0, len(levels))

	for _, level := range levels {
		tick, err := LevelTick(level)

		if err != nil {
			return nil, err
		}

		if math.IsNaN(level.Quantity) || math.IsInf(level.Quantity, 0) || level.Quantity < 0 {
			return nil, errnie.Error(errnie.Err(
				errnie.Validation,
				"-sample: level quantity must be finite and non-negative",
				nil,
			))
		}

		previous := sideBook.levels[tick]
		ops = append(ops, levelOp{
			tick:     tick,
			quantity: level.Quantity,
			previous: previous,
			touch:    sideBook.isTouchTick(tick),
		})
	}

	return ops, nil
}

func (sideBook *SideBook) commit(ops []levelOp) Frame {
	frame := Frame{}

	for _, op := range ops {
		if op.quantity == 0 {
			if op.previous > 0 {
				sideBook.remove(op.tick, op.previous)

				if op.touch {
					frame.touchCancel += op.previous
				}
			}

			continue
		}

		delta := op.quantity - op.previous
		sideBook.upsert(op.tick, op.previous, op.quantity)

		if delta <= 0 {
			if op.touch {
				frame.touchCancel += -delta
			}

			continue
		}

		frame.frameAdd += delta
	}

	return frame
}

/*
Apply validates then commits one side batch.
*/
func (sideBook *SideBook) Apply(levels []BookLevel, tickSize float64) (Frame, error) {
	_ = tickSize
	ops, err := sideBook.prepare(levels)

	if err != nil {
		return Frame{}, err
	}

	return sideBook.commit(ops), nil
}

func (sideBook *SideBook) upsert(tick int64, previous, quantity float64) {
	if previous == 0 {
		sideBook.ordered = append(sideBook.ordered, tick)
		sideBook.sortOrdered()
	}

	sideBook.levels[tick] = quantity
	sideBook.depth += quantity - previous
	sideBook.refreshBest()
}

func (sideBook *SideBook) remove(tick int64, previous float64) {
	delete(sideBook.levels, tick)
	sideBook.depth -= previous

	for index, orderedTick := range sideBook.ordered {
		if orderedTick != tick {
			continue
		}

		sideBook.ordered = append(sideBook.ordered[:index], sideBook.ordered[index+1:]...)

		break
	}

	sideBook.refreshBest()
}

func (sideBook *SideBook) sortOrdered() {
	if sideBook.side == SideBid {
		sort.Slice(sideBook.ordered, func(left, right int) bool {
			return sideBook.ordered[left] > sideBook.ordered[right]
		})

		return
	}

	sort.Slice(sideBook.ordered, func(left, right int) bool {
		return sideBook.ordered[left] < sideBook.ordered[right]
	})
}

func (sideBook *SideBook) refreshBest() {
	if len(sideBook.ordered) == 0 {
		sideBook.hasBest = false
		sideBook.best = 0

		return
	}

	sideBook.best = sideBook.ordered[0]
	sideBook.hasBest = true
}

/*
Best returns the touch price for this side.
*/
func (sideBook *SideBook) Best(tickSize float64) float64 {
	tick, ok := sideBook.bestTick()

	if !ok {
		return 0
	}

	return TickPrice(tick, tickSize)
}

/*
TouchQty returns quantity at the touch.
*/
func (sideBook *SideBook) TouchQty() float64 {
	tick, ok := sideBook.bestTick()

	if !ok {
		return 0
	}

	return sideBook.levels[tick]
}

/*
Depth returns total resting quantity.
*/
func (sideBook *SideBook) Depth() float64 {
	return sideBook.depth
}

/*
Notional returns quote-currency resting depth for this side.
*/
func (sideBook *SideBook) Notional(tickSize float64) float64 {
	notional := 0.0

	for tick, quantity := range sideBook.levels {
		notional += TickPrice(tick, tickSize) * quantity
	}

	return notional
}

/*
SideWeight decays resting quantity by distance from the midpoint.
*/
func (sideBook *SideBook) SideWeight(
	midTick float64,
	decayRate float64,
	touchOnly bool,
	flatDepth int,
) float64 {
	if touchOnly {
		return sideBook.TouchQty()
	}

	weight := 0.0
	remaining := flatDepth

	for _, tick := range sideBook.ordered {
		if flatDepth > 0 {
			if remaining <= 0 {
				break
			}

			remaining--
		}

		distance := math.Abs(float64(tick)-midTick) / midTick
		kernel := math.Exp(-decayRate * distance)
		weight += sideBook.levels[tick] * kernel
	}

	return weight
}

/*
Len returns resting level count.
*/
func (sideBook *SideBook) Len() int {
	return len(sideBook.levels)
}

func (sideBook *SideBook) bestTick() (int64, bool) {
	return sideBook.best, sideBook.hasBest
}

func (sideBook *SideBook) isTouchTick(tick int64) bool {
	bestTick, ok := sideBook.bestTick()

	return ok && bestTick == tick
}

func parseTickSize(tickSize float64) (*big.Rat, error) {
	if tickSize <= 0 || math.IsNaN(tickSize) || math.IsInf(tickSize, 0) {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: positive finite tick size required",
			nil,
		))
	}

	tick := new(big.Rat).SetFloat64(tickSize)

	if tick == nil {
		return nil, errnie.Error(errnie.Err(
			errnie.Validation,
			"-sample: tick size must convert to an exact rational",
			nil,
		))
	}

	return tick, nil
}

/*
sameTickSize reports whether two positive float ticks describe the same lattice
by exact rational equality after float→Rat conversion.
*/
func sameTickSize(left, right float64) bool {
	leftTick, leftErr := parseTickSize(left)
	rightTick, rightErr := parseTickSize(right)

	if leftErr != nil || rightErr != nil {
		return false
	}

	return leftTick.Cmp(rightTick) == 0
}
