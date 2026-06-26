package algorithm

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/hawkes"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/statistic"
)

const bivariateParamCount = 7

type excitationReading struct {
	frenzy             float64
	saturation         float64
	organic            float64
	exhaustion         float64
	strength           float64
	branchingRatio     float64
	stationarityMargin float64
	baselineMu         float64
	intensityRatio     float64
	poissonImprovement float64
	eventCount         int
	highRisk           bool
}

type excitationSymbol struct {
	fit                hawkes.BivariateFit
	hasFit             bool
	lastFitEventKey    fitEventKey
	lastFitTime        time.Time
	fitCooldown        time.Duration
	minFitEvents       int
	rawBase            *adaptive.EMA
	lastRawNorm        float64
	lastCategory       hawkes.FitCategory
	spectralRadii      []float64
	asymmetries        []float64
	bookTouchImbalance float64
}

type fitEventKey struct {
	buyCount            int
	sellCount           int
	buyFirst, buyLast   int64
	sellFirst, sellLast int64
}

func newExcitationSymbol() *excitationSymbol {
	return &excitationSymbol{
		minFitEvents: bivariateParamCount * 2,
		rawBase:      adaptive.NewEMA(datura.Acquire("excitation-ema", datura.APPJSON)),
	}
}

func (symbol *excitationSymbol) measure(
	buyTimes, sellTimes []time.Time,
	horizon time.Time,
	fitCooldown time.Duration,
	touchImbalance float64,
) (excitationReading, bool) {
	symbol.fitCooldown = fitCooldown
	symbol.bookTouchImbalance = touchImbalance

	stream := hawkes.NewArrivalStream(buyTimes, sellTimes)
	context, adaptiveStream, ok := fitContextFromStream(stream, horizon)

	if !ok || !context.EnoughEvents(adaptiveStream) {
		return excitationReading{}, false
	}

	fit, fitOk := symbol.fitForEvents(adaptiveStream, horizon)

	if !fitOk {
		return excitationReading{}, false
	}

	reading, readingOk := symbol.measureFit(fit)

	return symbol.enrichReading(reading, readingOk, fit, adaptiveStream, horizon)
}

func (symbol *excitationSymbol) fitForEvents(
	stream hawkes.ArrivalStream,
	horizon time.Time,
) (hawkes.BivariateFit, bool) {
	key := revisionKey(stream)

	if symbol.hasFit && key == symbol.lastFitEventKey {
		return symbol.fit.WithIntensitiesAt(stream, horizon), true
	}

	if symbol.hasFit &&
		symbol.fitCooldown > 0 &&
		!symbol.lastFitTime.IsZero() &&
		horizon.Sub(symbol.lastFitTime) < symbol.fitCooldown {
		return symbol.fit.WithIntensitiesAt(stream, horizon), true
	}

	fit := symbol.fitBivariate(stream, horizon)

	if fit.MuX <= 0 {
		return hawkes.BivariateFit{}, false
	}

	symbol.lastFitEventKey = key
	symbol.lastFitTime = horizon

	return fit, true
}

func (symbol *excitationSymbol) fitBivariate(
	stream hawkes.ArrivalStream,
	horizon time.Time,
) hawkes.BivariateFit {
	prior := hawkes.BivariateFit{}

	if symbol.hasFit {
		prior = symbol.fit
	}

	if context, ok := hawkes.NewFitContext(stream, horizon); ok {
		symbol.minFitEvents = context.MinFitEvents
	}

	fit := hawkes.NewBivariateEstimator(prior).Fit(stream, horizon)

	if fit.MuX > 0 {
		symbol.fit = fit
		symbol.hasFit = true
	}

	return fit
}

func (symbol *excitationSymbol) enrichReading(
	reading excitationReading,
	ok bool,
	fit hawkes.BivariateFit,
	stream hawkes.ArrivalStream,
	horizon time.Time,
) (excitationReading, bool) {
	if !ok {
		return excitationReading{}, false
	}

	reading.branchingRatio = fit.SpectralRadius
	reading.stationarityMargin = 1 - fit.SpectralRadius
	reading.baselineMu = fit.MuX + fit.MuY
	reading.intensityRatio = fit.IntensityX + fit.IntensityY

	if reading.baselineMu > 0 {
		reading.intensityRatio /= reading.baselineMu
	}

	reading.eventCount = len(stream.BuyTimes()) + len(stream.SellTimes())

	hawkesLikelihood := fit.LogLikelihood(stream, horizon)
	restricted := hawkes.BivariateFit{
		MuX: fit.MuX, MuY: fit.MuY,
		AlphaXX: fit.AlphaXX, AlphaYY: fit.AlphaYY, Beta: fit.Beta,
	}
	reading.poissonImprovement = hawkesLikelihood - restricted.LogLikelihood(stream, horizon)

	return reading, true
}

func (symbol *excitationSymbol) measureFit(fit hawkes.BivariateFit) (excitationReading, bool) {
	sellSide := fit.Asymmetry(true) > fit.Asymmetry(false)
	asymmetry := fit.Asymmetry(sellSide)
	asymmetry = confirmAsymmetryWithBook(asymmetry, sellSide, symbol.bookTouchImbalance)

	intensity, baseline := fit.IntensityX, fit.MuX

	if sellSide {
		intensity, baseline = fit.IntensityY, fit.MuY
	}

	raw := 1.0

	if baseline > 0 {
		raw = intensity / baseline
	}

	symbol.recordFitGates(fit.SpectralRadius, asymmetry)

	gates, gatesReady := hawkes.FitGatesFromHistory(symbol.spectralRadii, symbol.asymmetries)

	if !gatesReady {
		return excitationReading{}, false
	}

	category, confidence, err := hawkes.ClassifyFit(fit, asymmetry, sellSide, gates)

	if err != nil {
		return excitationReading{}, false
	}

	frenzy, saturation, organic, exhaustion := excitationTransitionScores(
		fit, asymmetry, sellSide, category, confidence, gates,
	)

	rawNorm := symbol.lastRawNorm
	symbol.lastRawNorm = symbol.rawBaseStep(raw)

	if rawNorm > 0 {
		saturationEvidence, err := competitionMargin(raw-rawNorm, rawNorm)

		if err == nil {
			saturationEvidence *= (1 - asymmetry)

			if saturationEvidence > confidence && category != hawkes.FitCategoryFrenzy {
				category = hawkes.FitCategorySaturation
				confidence = saturationEvidence
				saturation = saturationEvidence
			}
		}
	}

	if symbol.lastCategory == hawkes.FitCategoryFrenzy ||
		symbol.lastCategory == hawkes.FitCategorySaturation {
		exhaustionEvidence, err := competitionMargin(rawNorm-raw, rawNorm)

		if err == nil && exhaustionEvidence > confidence {
			category = hawkes.FitCategoryExhaustion
			confidence = exhaustionEvidence
			exhaustion = exhaustionEvidence
		}
	}

	symbol.lastCategory = category

	return excitationReading{
		strength:           raw,
		frenzy:             frenzy,
		saturation:         saturation,
		organic:            organic,
		exhaustion:         exhaustion,
		branchingRatio:     fit.SpectralRadius,
		stationarityMargin: 1 - fit.SpectralRadius,
		baselineMu:         fit.MuX + fit.MuY,
		intensityRatio:     raw,
		highRisk: category == hawkes.FitCategoryFrenzy ||
			category == hawkes.FitCategorySaturation,
	}, true
}

func (symbol *excitationSymbol) rawBaseStep(sample float64) float64 {
	inbound := datura.Acquire("excitation-ema-in", datura.APPJSON)
	inbound.WithPayload(datura.Map[any]{"sample": sample}.Marshal())
	frame := inbound.Pack()

	if len(frame) == 0 {
		return sample
	}

	_, _ = symbol.rawBase.Write(frame)
	out := make([]byte, 4096)
	readCount, _ := symbol.rawBase.Read(out)

	if readCount == 0 {
		return sample
	}

	outbound := datura.Acquire("excitation-ema-out", datura.Artifact_Type_json)
	_, _ = outbound.Unpack(out[:readCount])
	if outbound.HasPayload() {
		payload := outbound.DecryptPayload()
		value, ok := payloadScalar(payload)

		if ok {
			return value
		}
	}

	return sample
}

func (symbol *excitationSymbol) recordFitGates(spectralRadius, asymmetry float64) {
	if spectralRadius <= 0 {
		return
	}

	if math.IsNaN(asymmetry) || math.IsInf(asymmetry, 0) || asymmetry < 0 || asymmetry > 1 {
		return
	}

	capacity, err := fitGateHistoryCapacity(symbol.spectralRadii)

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"excitation: spectral radius history cap failed",
			err,
		))

		return
	}

	symbol.spectralRadii = appendRingFloat(symbol.spectralRadii, spectralRadius, capacity)

	capacity, err = fitGateHistoryCapacity(symbol.asymmetries)

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"excitation: asymmetry history cap failed",
			err,
		))

		return
	}

	symbol.asymmetries = appendRingFloat(symbol.asymmetries, asymmetry, capacity)
}

func fitGateHistoryCapacity(history []float64) (int, error) {
	minimumCap := bivariateParamCount * 2

	if len(history) < minimumCap {
		return minimumCap, nil
	}

	_, longWindow, err := statistic.ResolveWindows(history, 0, 0)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"excitation: fit gate window resolution failed",
			err,
		))
	}

	if longWindow < minimumCap {
		return minimumCap, nil
	}

	return longWindow, nil
}

func fitContextFromStream(
	stream hawkes.ArrivalStream,
	horizon time.Time,
) (hawkes.FitContext, hawkes.ArrivalStream, bool) {
	if len(stream.BuyTimes())+len(stream.SellTimes()) < 2 {
		return hawkes.FitContext{}, hawkes.ArrivalStream{}, false
	}

	probe, ok := hawkes.NewFitContext(stream, horizon)

	if !ok {
		return hawkes.FitContext{}, hawkes.ArrivalStream{}, false
	}

	adaptiveStart := horizon.Add(-probe.TradeWindow)
	adaptiveStream := clipStream(stream, adaptiveStart, horizon)
	context, ok := hawkes.NewFitContext(adaptiveStream, horizon)

	if !ok {
		return hawkes.FitContext{}, hawkes.ArrivalStream{}, false
	}

	return context, adaptiveStream, true
}

func clipStream(
	stream hawkes.ArrivalStream,
	windowStart, horizon time.Time,
) hawkes.ArrivalStream {
	buyTimes := clipTimes(stream.BuyTimes(), windowStart, horizon)
	sellTimes := clipTimes(stream.SellTimes(), windowStart, horizon)

	return hawkes.NewArrivalStream(buyTimes, sellTimes)
}

func clipTimes(times []time.Time, windowStart, horizon time.Time) []time.Time {
	clipped := make([]time.Time, 0, len(times))

	for _, wall := range times {
		if wall.Before(windowStart) || wall.After(horizon) {
			continue
		}

		clipped = append(clipped, wall)
	}

	return clipped
}

func revisionKey(stream hawkes.ArrivalStream) fitEventKey {
	buyTimes := stream.BuyTimes()
	sellTimes := stream.SellTimes()
	key := fitEventKey{
		buyCount:  len(buyTimes),
		sellCount: len(sellTimes),
	}

	if len(buyTimes) > 0 {
		key.buyFirst = buyTimes[0].UnixNano()
		key.buyLast = buyTimes[len(buyTimes)-1].UnixNano()
	}

	if len(sellTimes) > 0 {
		key.sellFirst = sellTimes[0].UnixNano()
		key.sellLast = sellTimes[len(sellTimes)-1].UnixNano()
	}

	return key
}

func excitationTransitionScores(
	fit hawkes.BivariateFit,
	asymmetry float64,
	sellSide bool,
	category hawkes.FitCategory,
	confidence float64,
	gates hawkes.FitGates,
) (frenzy, saturation, organic, exhaustion float64) {
	frenzy, saturation, organic, exhaustion = organicHeadroomScores(fit, asymmetry, sellSide, gates)

	switch category {
	case hawkes.FitCategorySaturation:
		saturation = math.Max(saturation, confidence)
	case hawkes.FitCategoryExhaustion:
		exhaustion = math.Max(exhaustion, confidence)
	case hawkes.FitCategoryFrenzy:
		frenzy = math.Max(frenzy, confidence)
	default:
		organic = math.Max(organic, confidence)
	}

	return frenzy, saturation, organic, exhaustion
}

func organicHeadroomScores(
	fit hawkes.BivariateFit,
	asymmetry float64,
	sellSide bool,
	gates hawkes.FitGates,
) (frenzy, saturation, organic, exhaustion float64) {
	if !gates.Ready() {
		return 0, 0, 0, 0
	}

	intensity, baseline := fit.IntensityX, fit.MuX

	if sellSide {
		intensity, baseline = fit.IntensityY, fit.MuY
	}

	headroom := -1.0
	saturationRadius := gates.SaturationRadius
	frenzyAsymmetry := gates.FrenzyAsymmetry

	if fit.SpectralRadius < saturationRadius {
		margin := saturationRadius - fit.SpectralRadius
		saturation, err := competitionMargin(margin, saturationRadius)

		if err == nil && saturation > headroom {
			headroom = saturation
		}
	}

	if baseline > 0 && intensity >= baseline && asymmetry < frenzyAsymmetry {
		margin := intensity - baseline
		organic = margin / (margin + baseline)

		if organic > headroom {
			headroom = organic
		}
	}

	if asymmetry < frenzyAsymmetry {
		margin := frenzyAsymmetry - asymmetry
		frenzy, err := competitionMargin(margin, frenzyAsymmetry)

		if err == nil && frenzy > headroom {
			headroom = frenzy
		}
	}

	if headroom < 0 {
		if baseline > 0 && intensity >= baseline {
			headroom = (intensity - baseline) / (intensity + baseline)
		}

		if headroom < 0 {
			return 0, 0, 0, 0
		}
	}

	return frenzy, saturation, organic, exhaustion
}

func competitionMargin(excess, span float64) (float64, error) {
	return probability.CompetitionMargin(excess, span)
}

func confirmAsymmetryWithBook(
	asymmetry float64,
	sellSide bool,
	touchImbalance float64,
) float64 {
	if math.IsNaN(touchImbalance) || math.IsInf(touchImbalance, 0) || touchImbalance == 0 {
		return asymmetry
	}

	bookMagnitude := math.Abs(touchImbalance)

	if bookMagnitude > 1 {
		bookMagnitude = 1
	}

	hawkesSign := 1.0

	if sellSide {
		hawkesSign = -1.0
	}

	bookSign := 1.0

	if touchImbalance < 0 {
		bookSign = -1.0
	}

	if hawkesSign*bookSign > 0 {
		return math.Min(1, asymmetry+bookMagnitude*(1-asymmetry))
	}

	return asymmetry * (1 - bookMagnitude)
}
