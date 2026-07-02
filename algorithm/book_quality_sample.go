package algorithm

import (
	"io"
	"math"
	"sort"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
)

const bookQualityFeatureCount = 13

/*
BookQualitySample turns order-book quality events into the feature vector
BookQuality expects. State is retained per symbol so concurrent symbols do not
share book, flow, or gate history.
*/
type BookQualitySample struct {
	artifact     *datura.Artifact
	windows      map[string]*bookQualityWindow
	pendingFrame bool
}

type bookQualityWindow struct {
	ledger          SideFlowLedger
	vacuumGate      *GateQuantile
	churnGate       *GateQuantile
	cancelQtyGate   *GateQuantile
	levelSizeGate   *GateQuantile
	vacuumLowPct    float64
	prevTouchAddBid float64
	prevTouchAddAsk float64
	bids            map[float64]float64
	asks            map[float64]float64
	bidOrders       map[string]bookQualityOrder
	askOrders       map[string]bookQualityOrder
	tradePrices     []float64
	frameCount      int
	lastFeatures    []float64
	lastBasis       float64
}

type bookQualityOrder struct {
	price float64
	qty   float64
}

/*
NewBookQualitySample returns a book encoder wired from a config artifact.
*/
func NewBookQualitySample(artifact *datura.Artifact) *BookQualitySample {
	return &BookQualitySample{
		artifact: artifact,
		windows:  map[string]*bookQualityWindow{},
	}
}

func (bookQualitySample *BookQualitySample) Write(payload []byte) (int, error) {
	bookQualitySample.artifact.WithPayload(payload)
	bookQualitySample.pendingFrame = true

	return len(payload), nil
}

func (bookQualitySample *BookQualitySample) Read(payload []byte) (int, error) {
	if !bookQualitySample.pendingFrame {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: no inbound frame",
			nil,
		))
	}

	bookQualitySample.pendingFrame = false

	state := datura.Acquire("book-quality-sample-state", datura.APPJSON)

	if _, err := state.Unpack(bookQualitySample.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: state write failed",
			err,
		))
	}

	defer state.Release()

	channel := datura.Peek[string](state, "channel")
	symbol := datura.Peek[string](state, "data", 0, "symbol")
	row := false

	if symbol == "" {
		symbol = datura.Peek[string](state, "symbol")
		row = symbol != ""
	}

	if symbol == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: symbol required",
			nil,
		))
	}

	if channel == "" && row {
		channel = bookQualityRowChannel(state)
	}

	window := bookQualitySample.window(symbol)

	switch channel {
	case "book":
		bookQualitySample.ingestBook(state, window, row)
	case "level3":
		bookQualitySample.ingestLevel3(state, window, row)
	case "trade":
		bookQualitySample.ingestTrade(state, window, row)

		return 0, io.EOF
	default:
		return 0, io.EOF
	}

	if len(window.lastFeatures) < bookQualityFeatureCount {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: insufficient features",
			nil,
		))
	}

	state.WithScope(symbol)
	state.Merge("features", window.lastFeatures)
	state.MergeOutput("l3", window.lastBasis)
	state.Poke("features", "root")
	state.Poke(equation.BookQualityInputKeys, "inputs")

	return state.PackInto(payload)
}

func (bookQualitySample *BookQualitySample) window(symbol string) *bookQualityWindow {
	if window, ok := bookQualitySample.windows[symbol]; ok {
		return window
	}

	window := &bookQualityWindow{
		vacuumGate: NewGateQuantile(
			datura.Acquire("vacuum-gate", datura.APPJSON).
				WithAttribute("percentile", datura.Peek[float64](bookQualitySample.artifact, "vacuumGate", "percentile")).
				WithAttribute("minSamples", datura.Peek[float64](bookQualitySample.artifact, "vacuumGate", "minSamples")),
		),
		churnGate: NewGateQuantile(
			datura.Acquire("churn-gate", datura.APPJSON).
				WithAttribute("percentile", datura.Peek[float64](bookQualitySample.artifact, "churnGate", "percentile")).
				WithAttribute("minSamples", datura.Peek[float64](bookQualitySample.artifact, "churnGate", "minSamples")),
		),
		cancelQtyGate: NewGateQuantile(
			datura.Acquire("cancel-qty-gate", datura.APPJSON).
				WithAttribute("percentile", datura.Peek[float64](bookQualitySample.artifact, "cancelQtyGate", "percentile")).
				WithAttribute("minSamples", datura.Peek[float64](bookQualitySample.artifact, "cancelQtyGate", "minSamples")),
		),
		levelSizeGate: NewGateQuantile(
			datura.Acquire("level-size-gate", datura.APPJSON).
				WithAttribute("percentile", datura.Peek[float64](bookQualitySample.artifact, "levelSizeGate", "percentile")).
				WithAttribute("minSamples", datura.Peek[float64](bookQualitySample.artifact, "levelSizeGate", "minSamples")),
		),
		vacuumLowPct: datura.Peek[float64](bookQualitySample.artifact, "vacuumLowPercentile"),
		bids:         map[float64]float64{},
		asks:         map[float64]float64{},
		bidOrders:    map[string]bookQualityOrder{},
		askOrders:    map[string]bookQualityOrder{},
	}

	bookQualitySample.windows[symbol] = window

	return window
}

func (bookQualitySample *BookQualitySample) ingestBook(
	state *datura.Artifact,
	window *bookQualityWindow,
	row bool,
) {
	frameCancelBid := 0.0
	frameFillBid := 0.0
	frameCancelAsk := 0.0
	frameFillAsk := 0.0
	frameAddBid := 0.0
	frameAddAsk := 0.0
	touchCancelBid := 0.0
	touchCancelAsk := 0.0

	bookQualitySample.applyLevels(
		state, window, "bids", SideBid, row,
		&frameCancelBid, &frameFillBid, &frameAddBid, &touchCancelBid,
	)
	bookQualitySample.applyLevels(
		state, window, "asks", SideAsk, row,
		&frameCancelAsk, &frameFillAsk, &frameAddAsk, &touchCancelAsk,
	)

	bookQualitySample.finishFrame(
		state, window,
		frameCancelBid, frameFillBid, frameCancelAsk, frameFillAsk,
		frameAddBid, frameAddAsk, touchCancelBid, touchCancelAsk,
		0,
	)
}

func (bookQualitySample *BookQualitySample) ingestLevel3(
	state *datura.Artifact,
	window *bookQualityWindow,
	row bool,
) {
	frameCancelBid := 0.0
	frameFillBid := 0.0
	frameCancelAsk := 0.0
	frameFillAsk := 0.0
	frameAddBid := 0.0
	frameAddAsk := 0.0
	touchCancelBid := 0.0
	touchCancelAsk := 0.0

	bookQualitySample.applyOrderEvents(
		state, window, "bids", SideBid, row,
		&frameCancelBid, &frameFillBid, &frameAddBid, &touchCancelBid,
	)
	bookQualitySample.applyOrderEvents(
		state, window, "asks", SideAsk, row,
		&frameCancelAsk, &frameFillAsk, &frameAddAsk, &touchCancelAsk,
	)

	bookQualitySample.finishFrame(
		state, window,
		frameCancelBid, frameFillBid, frameCancelAsk, frameFillAsk,
		frameAddBid, frameAddAsk, touchCancelBid, touchCancelAsk,
		1,
	)
	window.tradePrices = nil
}

func (bookQualitySample *BookQualitySample) ingestTrade(
	state *datura.Artifact,
	window *bookQualityWindow,
	row bool,
) {
	root := []any{"data", 0}
	if row {
		root = nil
	}

	price := bookQualityFloat(state, root, "price")

	if price <= 0 {
		return
	}

	window.tradePrices = append(window.tradePrices, price)
}

func (bookQualitySample *BookQualitySample) finishFrame(
	state *datura.Artifact,
	window *bookQualityWindow,
	frameCancelBid, frameFillBid, frameCancelAsk, frameFillAsk float64,
	frameAddBid, frameAddAsk, touchCancelBid, touchCancelAsk float64,
	basis float64,
) {
	window.frameCount++
	smoothing := 2.0 / float64(window.frameCount+1)

	if smoothing > 1 {
		smoothing = 1
	}

	window.ledger.ApplyFlow(SideBid, frameFillBid, frameCancelBid, smoothing)
	window.ledger.ApplyFlow(SideAsk, frameFillAsk, frameCancelAsk, smoothing)

	cancelBid, fillBid, cancelAsk, fillAsk, bidDepth, askDepth := window.ledger.Snapshot()
	bidRatio := bookQualityCancelFillRatio(cancelBid, fillBid)
	askRatio := bookQualityCancelFillRatio(cancelAsk, fillAsk)
	maxRatio := math.Max(bidRatio, askRatio)
	churnRatio := 0.0

	if frameAddBid > 0 && touchCancelBid > 0 {
		churnRatio = math.Max(churnRatio, touchCancelBid/frameAddBid)
	}

	if frameAddAsk > 0 && touchCancelAsk > 0 {
		churnRatio = math.Max(churnRatio, touchCancelAsk/frameAddAsk)
	}

	lastPrice := window.midPrice()
	threshold := window.runGate(window.vacuumGate, 0, 0)
	vacuumLow := window.runGate(window.vacuumGate, 0, window.resolvedVacuumLowPercentile())

	if threshold <= 0 && vacuumLow > 0 && maxRatio > vacuumLow {
		threshold = vacuumLow
	}

	vacuumPeak := window.runGate(window.vacuumGate, 0, 0)
	supportGate := supportRatioGate(threshold, vacuumLow, gateReady(window.vacuumGate.artifact))
	vacuumCap := vacuumStrengthLimit(threshold, maxRatio, vacuumPeak, gateReady(window.vacuumGate.artifact))

	if churnRatio <= 0 {
		if frameAddBid <= 0 && touchCancelBid > 0 && window.prevTouchAddBid > 0 {
			churnRatio = touchCancelBid / window.prevTouchAddBid
		}

		if frameAddAsk <= 0 && touchCancelAsk > 0 && window.prevTouchAddAsk > 0 {
			churnRatio = math.Max(churnRatio, touchCancelAsk/window.prevTouchAddAsk)
		}
	}

	if frameAddBid > 0 {
		window.prevTouchAddBid = frameAddBid
	}

	if frameAddAsk > 0 {
		window.prevTouchAddAsk = frameAddAsk
	}

	if maxRatio > 0 {
		window.runGate(window.vacuumGate, maxRatio, 0)
	}

	if churnRatio > 0 {
		window.runGate(window.churnGate, churnRatio, 0)
	}

	churnGate := window.runGate(window.churnGate, 0, 0)
	toxicNear := 0.0
	toxicBluffStrength := 0.0

	if lastPrice > 0 && churnRatio > 0 {
		proximity := window.touchProximity(lastPrice)
		sizeThreshold := largeBlockQtyThreshold(
			window.ledger.SideDepth(SideBid),
			window.medianLevelQty(state, "bids"),
			window.runGate(window.cancelQtyGate, 0, 0),
			window.runGate(window.levelSizeGate, 0, 0),
			gateReady(window.cancelQtyGate.artifact),
			gateReady(window.levelSizeGate.artifact),
		)
		bestBid := window.bestBid()
		bestAsk := window.bestAsk()

		if touchCancelBid > 0 {
			distance := math.Abs(bestBid-lastPrice) / lastPrice
			addVolume := frameAddBid

			if addVolume <= 0 {
				addVolume = window.prevTouchAddBid
			}

			evidence, err := ToxicChurnEvidence(
				churnRatio, churnGate, addVolume, sizeThreshold, distance, proximity,
			)

			if err == nil && evidence > 0 {
				toxicNear = 1
				toxicBluffStrength = math.Max(toxicBluffStrength, churnRatio)
			}
		}

		if touchCancelAsk > 0 {
			distance := math.Abs(bestAsk-lastPrice) / lastPrice
			addVolume := frameAddAsk

			if addVolume <= 0 {
				addVolume = window.prevTouchAddAsk
			}

			evidence, err := ToxicChurnEvidence(
				churnRatio, churnGate, addVolume, sizeThreshold, distance, proximity,
			)

			if err == nil && evidence > 0 {
				toxicNear = 1
				toxicBluffStrength = math.Max(toxicBluffStrength, churnRatio)
			}
		}
	}

	window.lastBasis = basis
	window.lastFeatures = []float64{
		cancelBid,
		fillBid,
		cancelAsk,
		fillAsk,
		bidDepth,
		askDepth,
		toxicNear,
		toxicBluffStrength,
		threshold,
		churnGate,
		supportGate,
		vacuumCap,
		lastPrice,
	}
}

func (bookQualitySample *BookQualitySample) applyLevels(
	state *datura.Artifact,
	window *bookQualityWindow,
	sideKey string,
	side byte,
	row bool,
	frameCancel, frameFill, frameAdd, touchCancel *float64,
) {
	book := window.sideBook(side)

	for index := 0; ; index++ {
		root := []any{"data", 0, sideKey, index}
		if row {
			root = []any{sideKey, index}
		}

		price := bookQualityFloat(state, root, "price")

		if price <= 0 {
			break
		}

		nextQty := bookQualityFloat(state, root, "qty")
		previousQty := book[price]
		delta := nextQty - previousQty

		if nextQty == 0 {
			touch := window.isTouchPrice(side, price)
			delete(book, price)
			window.ledger.AddDepth(side, -previousQty)
			*frameCancel += previousQty

			if touch {
				*touchCancel += previousQty
			}

			continue
		}

		book[price] = nextQty
		window.ledger.AddDepth(side, delta)

		if delta > 0 {
			*frameAdd += delta

			if previousQty > 0 && window.isTouchPrice(side, price) {
				*frameFill += delta
			}
		}

		if delta < 0 {
			removed := -delta
			*frameCancel += removed

			if window.isTouchPrice(side, price) {
				*touchCancel += removed
			}
		}
	}
}

func (bookQualitySample *BookQualitySample) applyOrderEvents(
	state *datura.Artifact,
	window *bookQualityWindow,
	sideKey string,
	side byte,
	row bool,
	frameCancel, frameFill, frameAdd, touchCancel *float64,
) {
	for index := 0; ; index++ {
		root := []any{"data", 0, sideKey, index}
		if row {
			root = []any{sideKey, index}
		}

		price := bookQualityFloat(state, root, "limit_price")

		if price <= 0 {
			break
		}

		orderID := bookQualityString(state, root, "order_id")

		if orderID == "" {
			continue
		}

		event := bookQualityString(state, root, "event")
		quantity := bookQualityFloat(state, root, "order_qty")

		if quantity <= 0 {
			continue
		}

		switch event {
		case "", "add", "modify":
			window.upsertOrder(side, orderID, price, quantity, frameAdd)
		case "delete":
			removed, touch := window.deleteOrder(side, orderID, price, quantity)

			if removed <= 0 {
				continue
			}

			if bookQualityTradedAt(price, window.tradePrices) {
				*frameFill += removed

				continue
			}

			*frameCancel += removed

			if touch {
				*touchCancel += removed
			}
		}
	}
}

func (window *bookQualityWindow) upsertOrder(
	side byte,
	orderID string,
	price float64,
	quantity float64,
	frameAdd *float64,
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

	orders[orderID] = bookQualityOrder{price: price, qty: quantity}
	book[price] += quantity
	window.ledger.AddDepth(side, quantity)

	if quantity > previous.qty {
		*frameAdd += quantity - previous.qty
	}
}

func (window *bookQualityWindow) deleteOrder(
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

func (window *bookQualityWindow) sideBook(side byte) map[float64]float64 {
	if side == SideBid {
		return window.bids
	}

	return window.asks
}

func (window *bookQualityWindow) sideOrders(side byte) map[string]bookQualityOrder {
	if side == SideBid {
		return window.bidOrders
	}

	return window.askOrders
}

func (window *bookQualityWindow) isTouchPrice(side byte, price float64) bool {
	if side == SideBid {
		return price == window.bestBid()
	}

	return price == window.bestAsk()
}

func (window *bookQualityWindow) bestBid() float64 {
	best := 0.0

	for price := range window.bids {
		if price > best {
			best = price
		}
	}

	return best
}

func (window *bookQualityWindow) bestAsk() float64 {
	best := 0.0

	for price := range window.asks {
		if best == 0 || price < best {
			best = price
		}
	}

	return best
}

func (window *bookQualityWindow) midPrice() float64 {
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

func (window *bookQualityWindow) touchProximity(mid float64) float64 {
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

func (window *bookQualityWindow) medianLevelQty(
	state *datura.Artifact,
	sideKey string,
) float64 {
	quantities := make([]float64, 0, 8)

	for index := 0; ; index++ {
		for _, key := range []string{"qty", "order_qty"} {
			qty := datura.Peek[float64](state, "data", 0, sideKey, index, key)

			if qty > 0 {
				quantities = append(quantities, qty)

				break
			}
		}

		price := datura.Peek[float64](state, "data", 0, sideKey, index, "price")
		limitPrice := datura.Peek[float64](state, "data", 0, sideKey, index, "limit_price")

		if price <= 0 && limitPrice <= 0 {
			break
		}
	}

	if len(quantities) == 0 {
		return 0
	}

	total := 0.0

	for _, qty := range quantities {
		total += qty
	}

	return total / float64(len(quantities))
}

func (window *bookQualityWindow) resolvedGatePercentile(configured float64, highSelectivity bool) float64 {
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

func (window *bookQualityWindow) resolvedVacuumLowPercentile() float64 {
	if window.vacuumLowPct > 0 {
		return window.vacuumLowPct
	}

	highBand := window.resolvedGatePercentile(0, true)

	return math.Max(0.1, highBand*0.25)
}

func (window *bookQualityWindow) runGate(
	gate *GateQuantile,
	sample float64,
	percentile float64,
) float64 {
	if percentile <= 0 {
		configured := datura.Peek[float64](gate.artifact, "percentile")

		if configured <= 0 {
			highSelectivity := gate == window.vacuumGate || gate == window.churnGate ||
				gate == window.levelSizeGate
			percentile = window.resolvedGatePercentile(0, highSelectivity)
		}
	}

	if sample <= 0 {
		return gate.value(percentile)
	}

	return gate.observe(sample, percentile)
}

func bookQualityRowChannel(state *datura.Artifact) string {
	if datura.Peek[float64](state, "price") > 0 &&
		datura.Peek[float64](state, "qty") > 0 {
		return "trade"
	}

	if datura.Peek[float64](state, "bids", 0, "limit_price") > 0 ||
		datura.Peek[float64](state, "asks", 0, "limit_price") > 0 {
		return "level3"
	}

	if datura.Peek[float64](state, "bids", 0, "price") > 0 ||
		datura.Peek[float64](state, "asks", 0, "price") > 0 {
		return "book"
	}

	return ""
}

func bookQualityFloat(state *datura.Artifact, root []any, key string) float64 {
	if len(root) == 0 {
		return datura.Peek[float64](state, key)
	}

	path := append(append([]any{}, root...), key)

	return datura.Peek[float64](state, path...)
}

func bookQualityString(state *datura.Artifact, root []any, key string) string {
	if len(root) == 0 {
		return datura.Peek[string](state, key)
	}

	path := append(append([]any{}, root...), key)

	return datura.Peek[string](state, path...)
}

func bookQualityCancelFillRatio(cancel, fill float64) float64 {
	if cancel <= 0 || fill <= 0 {
		return 0
	}

	return cancel / fill
}

func bookQualityTradedAt(price float64, trades []float64) bool {
	if price <= 0 || len(trades) == 0 {
		return false
	}

	tolerance := bookQualityMedianAbsoluteDeviation(trades)

	for _, traded := range trades {
		if math.Abs(traded-price) <= tolerance {
			return true
		}
	}

	return false
}

func bookQualityMedianAbsoluteDeviation(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	center := bookQualityMedian(values)
	deviations := make([]float64, 0, len(values))

	for _, value := range values {
		deviations = append(deviations, math.Abs(value-center))
	}

	return bookQualityMedian(deviations)
}

func bookQualityMedian(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2

	if len(sorted)%2 == 1 {
		return sorted[mid]
	}

	return (sorted[mid-1] + sorted[mid]) / 2
}

func (bookQualitySample *BookQualitySample) Close() error {
	return nil
}

var _ io.ReadWriteCloser = (*BookQualitySample)(nil)
