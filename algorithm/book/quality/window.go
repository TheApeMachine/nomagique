package quality

import (
	"math"

	"github.com/theapemachine/nomagique/algorithm/book/flow"
	"github.com/theapemachine/nomagique/equation"
)

type Window struct {
	ledger          flow.SideFlowLedger
	vacuumGate      *flow.GateQuantile
	churnGate       *flow.GateQuantile
	cancelQtyGate   *flow.GateQuantile
	levelSizeGate   *flow.GateQuantile
	vacuumLowPct    float64
	prevTouchAddBid float64
	prevTouchAddAsk float64
	bids            map[float64]float64
	asks            map[float64]float64
	bidOrders       map[string]Order
	askOrders       map[string]Order
	tradePrices     []float64
	frameCount      int
}

type Order struct {
	price float64
	qty   float64
}

func newWindow(config SampleConfig) *Window {
	return &Window{
		vacuumGate:    flow.NewGateQuantile(config.VacuumGate),
		churnGate:     flow.NewGateQuantile(config.ChurnGate),
		cancelQtyGate: flow.NewGateQuantile(config.CancelQtyGate),
		levelSizeGate: flow.NewGateQuantile(config.LevelSizeGate),
		vacuumLowPct:  config.VacuumLowPercentile,
		bids:          map[float64]float64{},
		asks:          map[float64]float64{},
		bidOrders:     map[string]Order{},
		askOrders:     map[string]Order{},
	}
}

func (window *Window) finish(
	frame Frame,
	level3 bool,
) equation.BookQualityInput {
	window.frameCount++
	smoothing := 2.0 / float64(window.frameCount+1)

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

func (window *Window) observeLevels(
	levels []flow.BookLevel,
	side byte,
	frame *Frame,
) {
	book := window.sideBook(side)

	for _, level := range levels {
		if level.Price <= 0 {
			return
		}

		previousQty := book[level.Price]
		delta := level.Quantity - previousQty
		touch := window.isTouchPrice(side, level.Price)

		if level.Quantity <= 0 {
			delete(book, level.Price)
			window.ledger.AddDepth(side, -previousQty)
			FrameCancel(frame, side, previousQty, touch)

			continue
		}

		book[level.Price] = level.Quantity
		window.ledger.AddDepth(side, delta)

		if delta > 0 {
			FrameAdd(frame, side, delta)

			if previousQty > 0 && touch {
				FrameFill(frame, side, delta)
			}

			continue
		}

		if delta < 0 {
			FrameCancel(frame, side, -delta, touch)
		}
	}
}

func (window *Window) observeOrderEvents(
	events []OrderEvent,
	side byte,
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

		removed, touch := window.deleteOrder(side, event.OrderID, event.Price, event.Quantity)

		if removed <= 0 {
			continue
		}

		if TradedAt(event.Price, window.tradePrices) {
			FrameFill(frame, side, removed)

			continue
		}

		FrameCancel(frame, side, removed, touch)
	}
}

func (window *Window) upsertOrder(
	side byte,
	orderID string,
	price float64,
	quantity float64,
	frame *Frame,
) {
	orders := window.sideOrders(side)
	book := window.sideBook(side)
	previous := orders[orderID]

	if previous.qty > 0 {
		book[previous.price] -= previous.qty

		if book[previous.price] <= 0 {
			delete(book, previous.price)
		}

		window.ledger.AddDepth(side, -previous.qty)
	}

	orders[orderID] = Order{price: price, qty: quantity}
	book[price] += quantity
	window.ledger.AddDepth(side, quantity)

	if quantity > previous.qty {
		FrameAdd(frame, side, quantity-previous.qty)
	}
}

func (window *Window) deleteOrder(
	side byte,
	orderID string,
	price float64,
	quantity float64,
) (float64, bool) {
	orders := window.sideOrders(side)
	book := window.sideBook(side)
	previous := orders[orderID]
	removed := quantity
	orderPrice := price

	if previous.qty > 0 {
		removed = previous.qty
		orderPrice = previous.price
	}

	touch := window.isTouchPrice(side, orderPrice)
	book[orderPrice] -= removed

	if book[orderPrice] <= 0 {
		delete(book, orderPrice)
	}

	window.ledger.AddDepth(side, -removed)
	delete(orders, orderID)

	return removed, touch
}
