package quality

import (
	"math"
	"sync"

	"github.com/theapemachine/nomagique/algorithm/book/flow"
)

func (window *Window) sideBook(side byte) *sync.Map {
	if side == flow.SideBid {
		return window.bids
	}

	return window.asks
}

func (window *Window) sideOrders(side byte) *sync.Map {
	if side == flow.SideBid {
		return window.bidOrders
	}

	return window.askOrders
}

func (window *Window) isTouchPrice(side byte, price float64) bool {
	if side == flow.SideBid {
		return price == window.bestBid()
	}

	return price == window.bestAsk()
}

func (window *Window) bestBid() float64 {
	best := 0.0

	window.bids.Range(func(key, value any) bool {
		price := key.(float64)

		if price > best {
			best = price
		}

		return true
	})

	return best
}

func (window *Window) bestAsk() float64 {
	best := 0.0

	window.asks.Range(func(key, value any) bool {
		price := key.(float64)

		if best == 0 || price < best {
			best = price
		}

		return true
	})

	return best
}

func (window *Window) midPrice() float64 {
	bestBid := window.bestBid()
	bestAsk := window.bestAsk()

	if bestBid > 0 && bestAsk > 0 {
		return (bestBid + bestAsk) / 2
	}

	if bestBid > 0 {
		return bestBid
	}

	return bestAsk
}

func (window *Window) threshold(maxRatio float64) float64 {
	threshold := window.vacuumGate.Value(0)
	vacuumLow := window.vacuumLow()

	if threshold <= 0 && vacuumLow > 0 && maxRatio > vacuumLow {
		return vacuumLow
	}

	return threshold
}

func (window *Window) vacuumLow() float64 {
	return window.vacuumGate.Value(window.resolvedVacuumLowPercentile())
}

func (window *Window) vacuumPeak() float64 {
	return window.vacuumGate.Value(0)
}

func (window *Window) churnRatio(frame Frame) float64 {
	churnRatio := 0.0

	if frame.addBid > 0 && frame.touchCancelBid > 0 {
		churnRatio = math.Max(churnRatio, frame.touchCancelBid/frame.addBid)
	}

	if frame.addAsk > 0 && frame.touchCancelAsk > 0 {
		churnRatio = math.Max(churnRatio, frame.touchCancelAsk/frame.addAsk)
	}

	if churnRatio <= 0 {
		churnRatio = math.Max(churnRatio, window.previousChurn(frame))
	}

	if frame.addBid > 0 {
		window.prevTouchAddBid = frame.addBid
	}

	if frame.addAsk > 0 {
		window.prevTouchAddAsk = frame.addAsk
	}

	return churnRatio
}

func (window *Window) previousChurn(frame Frame) float64 {
	churnRatio := 0.0

	if frame.addBid <= 0 && frame.touchCancelBid > 0 && window.prevTouchAddBid > 0 {
		churnRatio = frame.touchCancelBid / window.prevTouchAddBid
	}

	if frame.addAsk <= 0 && frame.touchCancelAsk > 0 && window.prevTouchAddAsk > 0 {
		churnRatio = math.Max(churnRatio, frame.touchCancelAsk/window.prevTouchAddAsk)
	}

	return churnRatio
}

func (window *Window) toxicity(
	frame Frame,
	churnRatio float64,
	churnGate float64,
	lastPrice float64,
) (bool, float64) {
	if lastPrice <= 0 || churnRatio <= 0 {
		return false, 0
	}

	proximity := window.touchProximity(lastPrice)
	sizeThreshold := flow.LargeBlockQtyThreshold(
		window.ledger.SideDepth(flow.SideBid),
		window.medianLevelQty(),
		window.cancelQtyGate.Value(0),
		window.levelSizeGate.Value(0),
		window.cancelQtyGate.Ready(),
		window.levelSizeGate.Ready(),
	)
	bestBid := window.bestBid()
	bestAsk := window.bestAsk()
	toxicNear := false
	toxicBluffStrength := 0.0

	toxicNear, toxicBluffStrength = window.toxicSide(
		frame.touchCancelBid, frame.addBid, window.prevTouchAddBid,
		bestBid, lastPrice, proximity, sizeThreshold, churnRatio, churnGate, toxicNear, toxicBluffStrength,
	)

	return window.toxicSide(
		frame.touchCancelAsk, frame.addAsk, window.prevTouchAddAsk,
		bestAsk, lastPrice, proximity, sizeThreshold, churnRatio, churnGate, toxicNear, toxicBluffStrength,
	)
}

func (window *Window) toxicSide(
	touchCancel float64,
	addVolume float64,
	previousAddVolume float64,
	price float64,
	lastPrice float64,
	proximity float64,
	sizeThreshold float64,
	churnRatio float64,
	churnGate float64,
	toxicNear bool,
	toxicBluffStrength float64,
) (bool, float64) {
	if touchCancel <= 0 {
		return toxicNear, toxicBluffStrength
	}

	if addVolume <= 0 {
		addVolume = previousAddVolume
	}

	distance := math.Abs(price-lastPrice) / lastPrice
	evidence, err := flow.ToxicChurnEvidence(
		churnRatio, churnGate, addVolume, sizeThreshold, distance, proximity,
	)

	if err != nil || evidence <= 0 {
		return toxicNear, toxicBluffStrength
	}

	return true, math.Max(toxicBluffStrength, churnRatio)
}

func (window *Window) touchProximity(mid float64) float64 {
	bestBid := window.bestBid()
	bestAsk := window.bestAsk()

	if mid <= 0 || bestBid <= 0 || bestAsk <= 0 {
		return 0
	}

	spread := (bestAsk - bestBid) / mid

	if spread <= 0 {
		return 0
	}

	return spread
}

func (window *Window) medianLevelQty() float64 {
	quantities := make([]float64, 0)

	window.bids.Range(func(key, value any) bool {
		qty := value.(float64)

		if qty > 0 {
			quantities = append(quantities, qty)
		}

		return true
	})

	window.asks.Range(func(key, value any) bool {
		qty := value.(float64)

		if qty > 0 {
			quantities = append(quantities, qty)
		}

		return true
	})

	return Median(quantities)
}

func (window *Window) resolvedGatePercentile(configured float64, highSelectivity bool) float64 {
	if configured > 0 {
		return configured
	}

	floor := 0.5
	ceiling := 0.75

	if highSelectivity {
		floor = 0.75
		ceiling = 0.9
	}

	if window.frameCount < 3 {
		return floor
	}

	ramp := math.Min(1, float64(window.frameCount-3)/17)

	return floor + (ceiling-floor)*ramp
}

func (window *Window) resolvedVacuumLowPercentile() float64 {
	if window.vacuumLowPct > 0 {
		return window.vacuumLowPct
	}

	highBand := window.resolvedGatePercentile(0, true)

	return math.Max(0.1, highBand*0.25)
}

func (window *Window) runGate(
	gate *flow.GateQuantile,
	sample float64,
	percentile float64,
) float64 {
	if percentile <= 0 && gate.ConfiguredPercentile() <= 0 {
		highSelectivity := gate == window.vacuumGate || gate == window.churnGate ||
			gate == window.levelSizeGate
		percentile = window.resolvedGatePercentile(0, highSelectivity)
	}

	if sample <= 0 {
		return gate.Value(percentile)
	}

	return gate.Observe(sample, percentile)
}
