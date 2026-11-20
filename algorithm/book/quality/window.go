package quality

import (
	"math"
	"sync"

	"github.com/theapemachine/nomagique/algorithm/book/flow"
	"github.com/theapemachine/nomagique/equation"
)

/*
Window retains cancel/fill ledgers, quantile gates, and level maps for one symbol's
book-quality sampling into equation.BookQualityInput.
*/
type Window struct {
	mu              sync.Mutex
	ledger          flow.SideFlowLedger
	vacuumGate      *flow.GateQuantile
	churnGate       *flow.GateQuantile
	cancelQtyGate   *flow.GateQuantile
	levelSizeGate   *flow.GateQuantile
	vacuumLowPct    float64
	prevTouchAddBid float64
	prevTouchAddAsk float64
	bids            *sync.Map
	asks            *sync.Map
	bidOrders       *sync.Map
	askOrders       *sync.Map
	tradePrices     []float64
	frameCount      int
}

const (
	flowSmoothingFloor = 2.0 / 33.0
	tradePriceCap      = 64
)

/*
Order tracks one resting order's price and quantity for L3 event attribution.
*/
type Order struct {
	price float64
	qty   float64
}

/*
newWindow wires quantile gates from sample config so thresholds warm with the tape.
*/
func newWindow(config SampleConfig) *Window {
	return &Window{
		vacuumGate:    flow.NewGateQuantile(config.VacuumGate),
		churnGate:     flow.NewGateQuantile(config.ChurnGate),
		cancelQtyGate: flow.NewGateQuantile(config.CancelQtyGate),
		levelSizeGate: flow.NewGateQuantile(config.LevelSizeGate),
		vacuumLowPct:  config.VacuumLowPercentile,
		bids:          &sync.Map{},
		asks:          &sync.Map{},
		bidOrders:     &sync.Map{},
		askOrders:     &sync.Map{},
	}
}

/*
finish smooths ledger flow, updates gates, and emits inputs for equation.BookQuality Measure.
*/
func (window *Window) finish(
	frame Frame,
	level3 bool,
) equation.BookQualityInput {
	window.frameCount++
	smoothing := 2.0 / float64(window.frameCount+1)

	if smoothing < flowSmoothingFloor {
		smoothing = flowSmoothingFloor
	}

	if smoothing > 1 {
		smoothing = 1
	}

	window.ledger.ApplyFlow(flow.SideBid, frame.fillBid, frame.cancelBid, smoothing)
	window.ledger.ApplyFlow(flow.SideAsk, frame.fillAsk, frame.cancelAsk, smoothing)

	cancelBid, fillBid, cancelAsk, fillAsk, bidDepth, askDepth := window.ledger.Snapshot()
	bidRatio := CancelFillRatio(cancelBid, fillBid)
	askRatio := CancelFillRatio(cancelAsk, fillAsk)
	maxRatio := math.Max(bidRatio, askRatio)
	churnRatio := window.churnRatio(frame)
	lastPrice := window.midPrice()
	threshold := window.threshold(maxRatio)
	churnGate := window.runGate(window.churnGate, churnRatio, 0)
	toxicNear, toxicBluffStrength := window.toxicity(frame, churnRatio, churnGate, lastPrice)

	if maxRatio > 0 {
		window.runGate(window.vacuumGate, maxRatio, 0)
	}

	return equation.BookQualityInput{
		CancelBid:          cancelBid,
		FillBid:            fillBid,
		CancelAsk:          cancelAsk,
		FillAsk:            fillAsk,
		BidDepth:           bidDepth,
		AskDepth:           askDepth,
		ToxicNear:          level3 && toxicNear,
		ToxicBluffStrength: toxicBluffStrength,
		Threshold:          threshold,
		ChurnGate:          churnGate,
		SupportGate:        flow.SupportRatioGate(threshold, window.vacuumLow(), window.vacuumGate.Ready()),
		VacuumStrengthCap:  flow.VacuumStrengthLimit(threshold, maxRatio, window.vacuumPeak(), window.vacuumGate.Ready()),
		LastPrice:          lastPrice,
	}
}

/*
snapshot reports the currently held book-quality state without recording a
new frame. A bare trade print carries no book delta of its own, so it
corroborates against — via tradePrices, consumed by the L3 fill/cancel
classification — rather than mutates the ledger's cancel/fill history.
*/
func (window *Window) snapshot() equation.BookQualityInput {
	cancelBid, fillBid, cancelAsk, fillAsk, bidDepth, askDepth := window.ledger.Snapshot()
	bidRatio := CancelFillRatio(cancelBid, fillBid)
	askRatio := CancelFillRatio(cancelAsk, fillAsk)
	maxRatio := math.Max(bidRatio, askRatio)
	lastPrice := window.midPrice()
	threshold := window.threshold(maxRatio)
	churnGate := window.runGate(window.churnGate, 0, 0)

	return equation.BookQualityInput{
		CancelBid:         cancelBid,
		FillBid:           fillBid,
		CancelAsk:         cancelAsk,
		FillAsk:           fillAsk,
		BidDepth:          bidDepth,
		AskDepth:          askDepth,
		Threshold:         threshold,
		ChurnGate:         churnGate,
		SupportGate:       flow.SupportRatioGate(threshold, window.vacuumLow(), window.vacuumGate.Ready()),
		VacuumStrengthCap: flow.VacuumStrengthLimit(threshold, maxRatio, window.vacuumPeak(), window.vacuumGate.Ready()),
		LastPrice:         lastPrice,
	}
}

/*
maturity reports a monotonically increasing, asymptotic confidence in the
window's cancel/fill ledger and gate thresholds as more book frames
accumulate. It never gates emission — finish() and snapshot() already emit a
defined value from the first frame — it only communicates how much evidence
backs it so far.
*/
func (window *Window) maturity() float64 {
	frameCount := float64(window.frameCount)

	return frameCount / (frameCount + 1)
}

/*
observeLevels translates level snapshots into frame cancel/fill/add for one side.
*/
func (window *Window) observeLevels(
	levels []flow.BookLevel,
	side flow.Side,
	frame *Frame,
) {
	book := window.sideBook(side)
	seen := make(map[float64]struct{}, len(levels))

	for _, level := range levels {
		if level.Price <= 0 {
			continue
		}

		seen[level.Price] = struct{}{}
		previousQty := 0.0

		if previous, ok := book.Load(level.Price); ok {
			previousQty = previous.(float64)
		}

		delta := level.Quantity - previousQty

		if level.Quantity <= 0 {
			book.Delete(level.Price)
			window.ledger.AddDepth(side, -previousQty)
			window.attributeRemoval(
				frame, side, level.Price, previousQty,
				window.nearCancelWeight(level.Price),
			)

			continue
		}

		book.Store(level.Price, level.Quantity)
		window.ledger.AddDepth(side, delta)

		if delta > 0 {
			FrameAdd(frame, side, delta)

			continue
		}

		if delta < 0 {
			window.attributeRemoval(
				frame, side, level.Price, -delta,
				window.nearCancelWeight(level.Price),
			)
		}
	}

	stale := make([]float64, 0)

	book.Range(func(key, value any) bool {
		price := key.(float64)

		if _, ok := seen[price]; !ok {
			stale = append(stale, price)
		}

		return true
	})

	for _, price := range stale {
		previous, ok := book.Load(price)

		if !ok {
			continue
		}

		book.Delete(price)
		qty := previous.(float64)
		window.ledger.AddDepth(side, -qty)
		window.attributeRemoval(
			frame, side, price, qty, window.nearCancelWeight(price),
		)
	}
}

/*
attributeRemoval records a size decrease as a fill only when a print landed there.
*/
func (window *Window) attributeRemoval(
	frame *Frame,
	side flow.Side,
	price float64,
	quantity float64,
	nearWeight float64,
) {
	if quantity <= 0 {
		return
	}

	if TradedAt(price, window.tradePrices) {
		FrameFill(frame, side, quantity)

		return
	}

	FrameCancel(frame, side, quantity, nearWeight)
}

/*
noteTrade keeps a bounded ring of recent print prices for fill corroboration.
*/
func (window *Window) noteTrade(price float64) {
	if len(window.tradePrices) < tradePriceCap {
		window.tradePrices = append(window.tradePrices, price)

		return
	}

	copy(window.tradePrices, window.tradePrices[1:])
	window.tradePrices[tradePriceCap-1] = price
}

/*
observeOrderEvents classifies L3 add/modify/delete against trade corroboration.
*/
func (window *Window) observeOrderEvents(
	events []OrderEvent,
	side flow.Side,
	frame *Frame,
) {
	for _, event := range events {
		if event.Price <= 0 {
			return
		}

		if event.OrderID == "" || event.Quantity <= 0 {
			continue
		}

		if event.Event == "" || event.Event == "add" || event.Event == "modify" {
			window.upsertOrder(side, event.OrderID, event.Price, event.Quantity, frame)

			continue
		}

		if event.Event != "delete" {
			continue
		}

		removed, orderPrice := window.deleteOrder(side, event.OrderID, event.Price, event.Quantity)

		if removed <= 0 {
			continue
		}

		if TradedAt(event.Price, window.tradePrices) {
			FrameFill(frame, side, removed)

			continue
		}

		FrameCancel(frame, side, removed, window.nearCancelWeight(orderPrice))
	}
}

/*
replaceOrders treats events as the full resting set and drops order ids the set omits.
*/
func (window *Window) replaceOrders(
	events []OrderEvent,
	side flow.Side,
	frame *Frame,
) {
	seen := make(map[string]struct{}, len(events))

	for _, event := range events {
		if event.Price <= 0 || event.OrderID == "" || event.Quantity <= 0 {
			continue
		}

		seen[event.OrderID] = struct{}{}
		window.upsertOrder(side, event.OrderID, event.Price, event.Quantity, frame)
	}

	orders := window.sideOrders(side)
	stale := make([]string, 0)

	orders.Range(func(key, value any) bool {
		id := key.(string)

		if _, ok := seen[id]; !ok {
			stale = append(stale, id)
		}

		return true
	})

	for _, id := range stale {
		existing, ok := orders.Load(id)

		if !ok {
			continue
		}

		order := existing.(Order)
		removed, orderPrice := window.deleteOrder(side, id, order.price, order.qty)

		if removed <= 0 {
			continue
		}

		if TradedAt(order.price, window.tradePrices) {
			FrameFill(frame, side, removed)

			continue
		}

		FrameCancel(frame, side, removed, window.nearCancelWeight(orderPrice))
	}
}

/*
upsertOrder moves order quantity between price levels while updating depth and frame adds.
*/
func (window *Window) upsertOrder(
	side flow.Side,
	orderID string,
	price float64,
	quantity float64,
	frame *Frame,
) {
	orders := window.sideOrders(side)
	book := window.sideBook(side)
	previous := Order{}

	if existing, ok := orders.Load(orderID); ok {
		previous = existing.(Order)
	}

	if previous.qty > 0 {
		previousQty := 0.0

		if existing, ok := book.Load(previous.price); ok {
			previousQty = existing.(float64)
		}

		newQty := previousQty - previous.qty

		if newQty <= 0 {
			book.Delete(previous.price)
		} else {
			book.Store(previous.price, newQty)
		}

		window.ledger.AddDepth(side, -previous.qty)
	}

	orders.Store(orderID, Order{price: price, qty: quantity})

	currentQty := 0.0

	if existing, ok := book.Load(price); ok {
		currentQty = existing.(float64)
	}

	book.Store(price, currentQty+quantity)
	window.ledger.AddDepth(side, quantity)

	if quantity > previous.qty {
		FrameAdd(frame, side, quantity-previous.qty)
	}
}

/*
deleteOrder removes resting quantity and returns the level price used for
distance-weighted cancel attribution.
*/
func (window *Window) deleteOrder(
	side flow.Side,
	orderID string,
	price float64,
	quantity float64,
) (float64, float64) {
	orders := window.sideOrders(side)
	book := window.sideBook(side)
	previous := Order{}

	if existing, ok := orders.Load(orderID); ok {
		previous = existing.(Order)
	}

	removed := quantity
	orderPrice := price

	if previous.qty > 0 {
		removed = previous.qty
		orderPrice = previous.price
	}

	currentQty := 0.0

	if existing, ok := book.Load(orderPrice); ok {
		currentQty = existing.(float64)
	}

	newQty := currentQty - removed

	if newQty <= 0 {
		book.Delete(orderPrice)
	} else {
		book.Store(orderPrice, newQty)
	}

	window.ledger.AddDepth(side, -removed)
	orders.Delete(orderID)

	return removed, orderPrice
}
