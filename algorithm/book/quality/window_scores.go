package quality

import (
	"math"
	"sync"

	"github.com/theapemachine/nomagique/algorithm/book/flow"
)

/*
sideBook returns the level map for a validated book side.
*/
func (window *Window) sideBook(side flow.Side) *sync.Map {
	if side == flow.SideBid {
		return window.bids
	}

	return window.asks
}

/*
sideOrders returns the order-id map for L3 attribution on one side.
*/
func (window *Window) sideOrders(side flow.Side) *sync.Map {
	if side == flow.SideBid {
		return window.bidOrders
	}

	return window.askOrders
}

/*
isTouchPrice reports whether a level price is currently at the touch on that side.
*/
func (window *Window) isTouchPrice(side flow.Side, price float64) bool {
	if side == flow.SideBid {
		return price == window.bestBid()
	}

	return price == window.bestAsk()
}

/*
nearCancelWeight is 1 at touch and decays exponentially with mid distance so
distant spoof cancels do not inflate bluff churn (Fabre & Challet).
*/
func (window *Window) nearCancelWeight(price float64) float64 {
	if price <= 0 {
		return 0
	}

	bid := window.bestBid()
	ask := window.bestAsk()
	if bid <= 0 || ask <= 0 || ask <= bid {
		if window.isTouchPrice(flow.SideBid, price) || window.isTouchPrice(flow.SideAsk, price) {
			return 1
		}
		return 0
	}

	mid := 0.5 * (bid + ask)
	if mid <= 0 {
		return 0
	}

	spread := ask - bid
	decay := flow.DecayRate(mid, spread)
	distance := math.Abs(price-mid) / mid
	weight := math.Exp(-decay * distance)
	if weight > 1 {
		return 1
	}
	if weight < 0 {
		return 0
	}
	return weight
}

/*
bestBid scans resting bid levels for the highest touch price.
*/
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

/*
bestAsk scans resting ask levels for the lowest touch price.
*/
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

/*
midPrice derives lastPrice for equation inputs from the current two-sided touch.
*/
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

/*
threshold resolves the active vacuum gate, falling back to empirical low band when cold.
*/
func (window *Window) threshold(maxRatio float64) float64 {
	threshold := window.vacuumGate.Value(0)
	vacuumLow := window.vacuumLow()

	if threshold <= 0 && vacuumLow > 0 && maxRatio > vacuumLow {
		return vacuumLow
	}

	return threshold
}

/*
vacuumLow reads the lower vacuum quantile used for support gate derivation.
*/
func (window *Window) vacuumLow() float64 {
	return window.vacuumGate.Value(window.resolvedVacuumLowPercentile())
}

/*
vacuumPeak reads the peak vacuum ratio retained for strength caps in equation inputs.
*/
func (window *Window) vacuumPeak() float64 {
	return window.vacuumGate.Value(0)
}

/*
churnRatio scores touch cancel relative to touch adds for bluff detection this frame.
*/
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

/*
previousChurn estimates churn when this frame lacks adds but still cancels at touch.
*/
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

/*
toxicity fuses bid and ask bluff evidence for ToxicNear and ToxicBluffStrength inputs.
*/
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

/*
toxicSide scores one side's touch churn against size and proximity for bluff export.
*/
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

/*
touchProximity converts quoted spread into a relative near-touch band for toxicity.
*/
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

/*
medianLevelQty summarizes typical resting size for large-block cancel thresholds.
*/
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

/*
resolvedGatePercentile ramps quantile selectivity as frames accumulate stable evidence.
*/
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

/*
resolvedVacuumLowPercentile anchors support gates to a conservative vacuum quantile.
*/
func (window *Window) resolvedVacuumLowPercentile() float64 {
	if window.vacuumLowPct > 0 {
		return window.vacuumLowPct
	}

	highBand := window.resolvedGatePercentile(0, true)

	return math.Max(0.1, highBand*0.25)
}

/*
runGate observes a sample when present otherwise returns the current gate value.
*/
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
