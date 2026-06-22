package algorithm

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

const bookQualityFeatureCount = 13

/*
BookQualitySample turns Kraken book frames into the feature vector BookQuality expects.
Ledger and gate state live on the stage instance across Measure calls.
*/
type BookQualitySample struct {
	artifact      *datura.Artifact
	ledger        SideFlowLedger
	vacuumGate    *GateQuantile
	churnGate     *GateQuantile
	cancelQtyGate *GateQuantile
	levelSizeGate *GateQuantile
	fillMatchGate *GateQuantile
	bids          map[float64]float64
	asks          map[float64]float64
	frameCount    int
	pendingFrame  bool
}

/*
NewBookQualitySample returns a book encoder wired from a config artifact.
*/
func NewBookQualitySample(artifact *datura.Artifact) *BookQualitySample {
	return &BookQualitySample{
		artifact: artifact,
		vacuumGate: NewGateQuantile(
			datura.Acquire("vacuum-gate", datura.APPJSON).
				WithAttribute("percentile", datura.Peek[float64](artifact, "vacuumGate", "percentile")).
				WithAttribute("minSamples", datura.Peek[float64](artifact, "vacuumGate", "minSamples")),
		),
		churnGate: NewGateQuantile(
			datura.Acquire("churn-gate", datura.APPJSON).
				WithAttribute("percentile", datura.Peek[float64](artifact, "churnGate", "percentile")).
				WithAttribute("minSamples", datura.Peek[float64](artifact, "churnGate", "minSamples")),
		),
		cancelQtyGate: NewGateQuantile(
			datura.Acquire("cancel-qty-gate", datura.APPJSON).
				WithAttribute("percentile", datura.Peek[float64](artifact, "cancelQtyGate", "percentile")).
				WithAttribute("minSamples", datura.Peek[float64](artifact, "cancelQtyGate", "minSamples")),
		),
		levelSizeGate: NewGateQuantile(
			datura.Acquire("level-size-gate", datura.APPJSON).
				WithAttribute("percentile", datura.Peek[float64](artifact, "levelSizeGate", "percentile")).
				WithAttribute("minSamples", datura.Peek[float64](artifact, "levelSizeGate", "minSamples")),
		),
		fillMatchGate: NewGateQuantile(
			datura.Acquire("fill-match-gate", datura.APPJSON).
				WithAttribute("percentile", datura.Peek[float64](artifact, "fillMatchGate", "percentile")).
				WithAttribute("minSamples", datura.Peek[float64](artifact, "fillMatchGate", "minSamples")),
		),
		bids: map[float64]float64{},
		asks: map[float64]float64{},
	}
}

func (bookQualitySample *BookQualitySample) Write(payload []byte) (int, error) {
	bookQualitySample.artifact.WithPayload(payload)
	bookQualitySample.pendingFrame = true

	return len(payload), nil
}

func (bookQualitySample *BookQualitySample) Read(payload []byte) (int, error) {
	if !bookQualitySample.pendingFrame {
		return 0, io.EOF
	}

	bookQualitySample.pendingFrame = false

	state := datura.Acquire("book-quality-sample-state", datura.APPJSON)

	if _, err := state.Write(bookQualitySample.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	channel := datura.Peek[string](state, "channel")

	if channel == "book" {
		bookQualitySample.ingestBook(state)
	}

	features := datura.Peek[[]float64](bookQualitySample.artifact, "lastFeatures")

	if len(features) < bookQualityFeatureCount {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"book-quality-sample: insufficient features",
			nil,
		))
	}

	state.Merge("features", features)
	state.Merge("root", "features")
	state.Merge("inputs", []string{"features"})

	return state.Read(payload)
}

func (bookQualitySample *BookQualitySample) ingestBook(state *datura.Artifact) {
	frameCancelBid := 0.0
	frameFillBid := 0.0
	frameCancelAsk := 0.0
	frameFillAsk := 0.0
	frameAddBid := 0.0
	frameAddAsk := 0.0
	touchCancelBid := 0.0
	touchCancelAsk := 0.0

	bookQualitySample.applyLevels(
		state, "bids", SideBid,
		&frameCancelBid, &frameFillBid, &frameAddBid, &touchCancelBid,
	)

	bookQualitySample.applyLevels(
		state, "asks", SideAsk,
		&frameCancelAsk, &frameFillAsk, &frameAddAsk, &touchCancelAsk,
	)

	bookQualitySample.frameCount++
	smoothing := 2.0 / float64(bookQualitySample.frameCount+1)

	if smoothing > 1 {
		smoothing = 1
	}

	bookQualitySample.ledger.ApplyFlow(SideBid, frameFillBid, frameCancelBid, smoothing)
	bookQualitySample.ledger.ApplyFlow(SideAsk, frameFillAsk, frameCancelAsk, smoothing)

	cancelBid, fillBid, cancelAsk, fillAsk, bidDepth, askDepth := bookQualitySample.ledger.Snapshot()
	maxRatio := math.Max(
		CancelFillRatio(cancelBid, fillBid),
		CancelFillRatio(cancelAsk, fillAsk),
	)

	churnRatio := 0.0

	if frameAddBid > 0 && touchCancelBid > 0 {
		churnRatio = math.Max(churnRatio, touchCancelBid/frameAddBid)
	}

	if frameAddAsk > 0 && touchCancelAsk > 0 {
		churnRatio = math.Max(churnRatio, touchCancelAsk/frameAddAsk)
	}

	lastPrice := bookQualitySample.midPrice()
	threshold := bookQualitySample.runGate(bookQualitySample.vacuumGate, 0, 0)

	if threshold <= 0 && maxRatio > 0 {
		threshold = maxRatio
	}

	churnGate := bookQualitySample.runGate(bookQualitySample.churnGate, 0, 0)
	vacuumPeak := bookQualitySample.runGate(bookQualitySample.vacuumGate, 0, 0)
	vacuumLow := bookQualitySample.runGate(
		bookQualitySample.vacuumGate,
		0,
		datura.Peek[float64](bookQualitySample.artifact, "vacuumLowPercentile"),
	)
	supportGate := supportRatioGate(
		threshold,
		vacuumLow,
		gateReady(bookQualitySample.vacuumGate.artifact),
	)
	vacuumCap := vacuumStrengthLimit(
		threshold,
		maxRatio,
		vacuumPeak,
		gateReady(bookQualitySample.vacuumGate.artifact),
	)

	if maxRatio > 0 {
		bookQualitySample.runGate(bookQualitySample.vacuumGate, maxRatio, 0)
	}

	if churnRatio > 0 {
		bookQualitySample.runGate(bookQualitySample.churnGate, churnRatio, 0)
	}
	toxicNear := 0.0
	toxicBluffStrength := 0.0

	if lastPrice > 0 && churnGate > 0 {
		proximity := bookQualitySample.touchProximity(lastPrice)
		sizeThreshold := largeBlockQtyThreshold(
			bookQualitySample.ledger.SideDepth(SideBid),
			bookQualitySample.medianLevelQty(state, "bids"),
			bookQualitySample.runGate(bookQualitySample.cancelQtyGate, 0, 0),
			bookQualitySample.runGate(bookQualitySample.levelSizeGate, 0, 0),
			gateReady(bookQualitySample.cancelQtyGate.artifact),
			gateReady(bookQualitySample.levelSizeGate.artifact),
		)
		bestBid := bookQualitySample.bestBid()
		bestAsk := bookQualitySample.bestAsk()

		if touchCancelBid > 0 {
			distance := math.Abs(bestBid-lastPrice) / lastPrice
			evidence, evidenceErr := ToxicChurnEvidence(
				churnRatio, churnGate, frameAddBid, sizeThreshold, distance, proximity,
			)

			if evidenceErr == nil && evidence > 0 {
				toxicNear = 1
				toxicBluffStrength = math.Max(toxicBluffStrength, churnRatio)
			}
		}

		if touchCancelAsk > 0 {
			distance := math.Abs(bestAsk-lastPrice) / lastPrice
			evidence, evidenceErr := ToxicChurnEvidence(
				churnRatio, churnGate, frameAddAsk, sizeThreshold, distance, proximity,
			)

			if evidenceErr == nil && evidence > 0 {
				toxicNear = 1
				toxicBluffStrength = math.Max(toxicBluffStrength, churnRatio)
			}
		}
	}

	features := []float64{
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

	bookQualitySample.artifact.Poke(features, "lastFeatures")
}

func (bookQualitySample *BookQualitySample) applyLevels(
	state *datura.Artifact,
	sideKey string,
	side byte,
	frameCancel, frameFill, frameAdd, touchCancel *float64,
) {
	book := bookQualitySample.sideBook(side)

	for index := 0; ; index++ {
		price := datura.Peek[float64](state, "data", 0, sideKey, index, "price")

		if price <= 0 {
			break
		}

		nextQty := datura.Peek[float64](state, "data", 0, sideKey, index, "qty")
		previousQty := book[price]
		delta := nextQty - previousQty

		if nextQty == 0 {
			delete(book, price)
			*frameCancel += previousQty

			if bookQualitySample.isTouchPrice(side, price) {
				*touchCancel += previousQty
			}

			continue
		}

		book[price] = nextQty
		bookQualitySample.ledger.AddDepth(side, delta)

		if delta > 0 {
			*frameAdd += delta

			if previousQty > 0 && bookQualitySample.isTouchPrice(side, price) {
				*frameFill += delta
			}
		}

		if delta < 0 {
			removed := -delta
			*frameCancel += removed

			if bookQualitySample.isTouchPrice(side, price) {
				*touchCancel += removed
			}
		}
	}
}

func (bookQualitySample *BookQualitySample) isTouchPrice(side byte, price float64) bool {
	if side == SideBid {
		return price == bookQualitySample.bestBid()
	}

	return price == bookQualitySample.bestAsk()
}

func (bookQualitySample *BookQualitySample) sideBook(side byte) map[float64]float64 {
	if side == SideBid {
		return bookQualitySample.bids
	}

	return bookQualitySample.asks
}

func (bookQualitySample *BookQualitySample) bestBid() float64 {
	best := 0.0

	for price := range bookQualitySample.bids {
		if price > best {
			best = price
		}
	}

	return best
}

func (bookQualitySample *BookQualitySample) bestAsk() float64 {
	best := 0.0

	for price := range bookQualitySample.asks {
		if best == 0 || price < best {
			best = price
		}
	}

	return best
}

func (bookQualitySample *BookQualitySample) midPrice() float64 {
	bestBid := bookQualitySample.bestBid()
	bestAsk := bookQualitySample.bestAsk()

	if bestBid > 0 && bestAsk > 0 {
		return (bestBid + bestAsk) / 2
	}

	if bestBid > 0 {
		return bestBid
	}

	return bestAsk
}

func (bookQualitySample *BookQualitySample) touchProximity(mid float64) float64 {
	bestBid := bookQualitySample.bestBid()
	bestAsk := bookQualitySample.bestAsk()

	if mid <= 0 || bestBid <= 0 || bestAsk <= 0 {
		return 0
	}

	spread := (bestAsk - bestBid) / mid

	if spread <= 0 {
		return 0
	}

	return spread
}

func (bookQualitySample *BookQualitySample) medianLevelQty(
	state *datura.Artifact,
	sideKey string,
) float64 {
	quantities := make([]float64, 0, 8)

	for index := 0; ; index++ {
		qty := datura.Peek[float64](state, "data", 0, sideKey, index, "qty")

		if qty <= 0 {
			break
		}

		quantities = append(quantities, qty)
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

func (bookQualitySample *BookQualitySample) runGate(
	gate *GateQuantile,
	sample float64,
	percentile float64,
) float64 {
	if sample <= 0 {
		return gate.value(percentile)
	}

	return gate.observe(sample, percentile)
}

func (bookQualitySample *BookQualitySample) Close() error {
	return nil
}

var _ io.ReadWriteCloser = (*BookQualitySample)(nil)
