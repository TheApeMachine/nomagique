package algorithm

import (
	"io"
	"math"
	"sync"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/hawkes"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/statistic"
)

const (
	excitationPayloadHeader = 5
	bivariateParamCount     = 7
	hawkesFitCooldownMult   = 50
)

var ExcitationSampleInputKeys = []string{
	"horizonSeconds",
	"fitCooldownSeconds",
	"buyCount",
	"sellCount",
	"touchImbalance",
}

/*
ExcitationOutcome holds Hawkes thermal scores from a bivariate fit.
*/
type ExcitationOutcome struct {
	Frenzy             float64
	Saturation         float64
	Organic            float64
	Exhaustion         float64
	Strength           float64
	BranchingRatio     float64
	StationarityMargin float64
	BaselineMu         float64
	IntensityRatio     float64
	PoissonImprovement float64
	EventCount         int
	HighRisk           bool
	Eligible           bool
}

/*
Excitation fits a bivariate Hawkes process over buy/sell arrival times.

Payload layout: horizonSeconds, fitCooldownSeconds, buyCount, sellCount,
touchImbalance, then buy arrival seconds, then sell arrival seconds.
Per-scope fit state is keyed from the artifact scope attribute.
*/
type Excitation struct {
	artifact *datura.Artifact
	symbols  sync.Map
	outcome  ExcitationOutcome
}

/*
NewExcitation returns a Hawkes excitation stage wired from config on the artifact.
*/
func NewExcitation(artifact *datura.Artifact) *Excitation {
	return &Excitation{
		artifact: artifact,
	}
}

func (excitation *Excitation) Write(payload []byte) (int, error) {
	excitation.artifact.WithPayload(payload)
	return len(payload), nil
}

func (excitation *Excitation) Read(payload []byte) (int, error) {
	state := datura.Acquire("excitation-state", datura.APPJSON)

	if _, err := state.Write(excitation.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	state.Inspect("algorithm", "excitation", "Read()", "p")

	scope, _ := state.Scope()

	if scope == "" {
		scope = datura.Peek[string](state, "scope")
	}

	if scope == "" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"excitation: scope required",
			nil,
		))
	}

	outcome, err := excitation.evaluateState(state, scope)

	if err != nil {
		return 0, err
	}

	excitation.outcome = outcome

	state.MergeOutput("frenzy", excitation.outcome.Frenzy)
	state.MergeOutput("saturation", excitation.outcome.Saturation)
	state.MergeOutput("organic", excitation.outcome.Organic)
	state.MergeOutput("exhaustion", excitation.outcome.Exhaustion)
	state.MergeOutput("strength", excitation.outcome.Strength)
	state.MergeOutput("branchingRatio", excitation.outcome.BranchingRatio)
	state.MergeOutput("spectralRadius", excitation.outcome.BranchingRatio)
	state.MergeOutput("stationarityMargin", excitation.outcome.StationarityMargin)
	state.MergeOutput("baselineMu", excitation.outcome.BaselineMu)
	state.MergeOutput("intensityRatio", excitation.outcome.IntensityRatio)

	if excitation.outcome.Eligible {
		state.Merge("excitation.eligible", 1.0)
	}

	state.Poke("output", "root")
	state.Poke([]string{"frenzy", "saturation", "organic", "exhaustion"}, "inputs")

	return state.Read(payload)
}

func (excitation *Excitation) Close() error {
	return nil
}

/*
Outcome returns the thermal scores from the last Read.
*/
func (excitation *Excitation) Outcome() ExcitationOutcome {
	return excitation.outcome
}

func (excitation *Excitation) evaluateState(
	state *datura.Artifact,
	scope string,
) (ExcitationOutcome, error) {
	inputKeys := equation.EnsureFeatureSchema(state, excitation.artifact, ExcitationSampleInputKeys)
	header, err := equation.FeatureFields(state, inputKeys)

	if err != nil {
		return ExcitationOutcome{}, err
	}

	buyCount := int(header[2])
	sellCount := int(header[3])
	tail, err := equation.FeatureSlice(state, excitationPayloadHeader, buyCount+sellCount)

	if err != nil {
		return ExcitationOutcome{}, err
	}

	batch := append(append([]float64(nil), header...), tail...)

	return excitation.evaluate(scope, batch)
}

func (excitation *Excitation) evaluate(scope string, batch []float64) (ExcitationOutcome, error) {
	buyCount, _, expectedLen, batchOK := excitationBatchBounds(batch)

	if scope == "" || !batchOK {
		return ExcitationOutcome{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"excitation: invalid batch",
			nil,
		))
	}

	horizon := time.Unix(0, int64(batch[0]*float64(time.Second)))
	fitCooldown := time.Duration(batch[1] * float64(time.Second))

	buyTimes := secondsToTimes(batch[excitationPayloadHeader : excitationPayloadHeader+buyCount])
	sellTimes := secondsToTimes(batch[excitationPayloadHeader+buyCount : expectedLen])
	touchImbalance := batch[4]

	symbolState := excitation.loadSymbol(scope)
	reading, ok := symbolState.measure(buyTimes, sellTimes, horizon, fitCooldown, touchImbalance)

	if !ok {
		return ExcitationOutcome{}, io.EOF
	}

	return excitationOutcomeFromReading(reading), nil
}

func (excitation *Excitation) loadSymbol(scope string) *excitationSymbol {
	value, _ := excitation.symbols.LoadOrStore(scope, newExcitationSymbol())

	return value.(*excitationSymbol)
}

func (excitation *Excitation) FrenzyReading() *ExcitationReading {
	return newExcitationReading(excitation, func(outcome ExcitationOutcome) float64 {
		return outcome.Frenzy
	})
}

func (excitation *Excitation) SaturationReading() *ExcitationReading {
	return newExcitationReading(excitation, func(outcome ExcitationOutcome) float64 {
		return outcome.Saturation
	})
}

func (excitation *Excitation) OrganicReading() *ExcitationReading {
	return newExcitationReading(excitation, func(outcome ExcitationOutcome) float64 {
		return outcome.Organic
	})
}

func (excitation *Excitation) ExhaustionReading() *ExcitationReading {
	return newExcitationReading(excitation, func(outcome ExcitationOutcome) float64 {
		return outcome.Exhaustion
	})
}

/*
ExcitationReading exposes one ExcitationOutcome field as a pipeline score source.
*/
type ExcitationReading struct {
	artifact   *datura.Artifact
	excitation *Excitation
	project    func(ExcitationOutcome) float64
}

func newExcitationReading(
	excitation *Excitation,
	project func(ExcitationOutcome) float64,
) *ExcitationReading {
	return &ExcitationReading{
		artifact:   datura.Acquire("excitation-reading", datura.Artifact_Type_json),
		excitation: excitation,
		project:    project,
	}
}

func (reading *ExcitationReading) Write(p []byte) (int, error) {
	reading.artifact.WithPayload(p)
	return len(p), nil
}

func (reading *ExcitationReading) Read(payload []byte) (int, error) {
	state := datura.Acquire("excitation-reading-state", datura.APPJSON)

	if _, err := state.Write(reading.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	value := 0.0

	if reading.excitation != nil && reading.project != nil {
		value = reading.project(reading.excitation.outcome)
	}

	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.Read(payload)
}

func (reading *ExcitationReading) Close() error {
	return nil
}

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

	category, confidence, classifyErr := hawkes.ClassifyFit(fit, asymmetry, sellSide, gates)

	if classifyErr != nil {
		return excitationReading{}, false
	}

	frenzy, saturation, organic, exhaustion := excitationTransitionScores(
		fit, asymmetry, sellSide, category, confidence, gates,
	)

	rawNorm := symbol.lastRawNorm
	symbol.lastRawNorm = symbol.rawBaseStep(raw)

	if rawNorm > 0 {
		saturationEvidence, marginErr := competitionMargin(raw-rawNorm, rawNorm)

		if marginErr == nil {
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
		exhaustionEvidence, marginErr := competitionMargin(rawNorm-raw, rawNorm)

		if marginErr == nil && exhaustionEvidence > confidence {
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
	_, _ = outbound.Write(out[:readCount])
	if outbound.HasEncryptedPayload() {
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

	symbol.spectralRadii = appendRingFloat(symbol.spectralRadii, spectralRadius, fitGateHistoryCap(symbol.spectralRadii))
	symbol.asymmetries = appendRingFloat(symbol.asymmetries, asymmetry, fitGateHistoryCap(symbol.asymmetries))
}

func fitGateHistoryCap(history []float64) int {
	minimumCap := bivariateParamCount * 2

	if len(history) < minimumCap {
		return minimumCap
	}

	_, longWindow, err := statistic.NewRollingWindow(0, 0).Resolve(history)

	if err != nil {
		return minimumCap
	}

	if longWindow < minimumCap {
		return minimumCap
	}

	return longWindow
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
		saturation, marginErr := competitionMargin(margin, saturationRadius)

		if marginErr == nil && saturation > headroom {
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
		frenzy, marginErr := competitionMargin(margin, frenzyAsymmetry)

		if marginErr == nil && frenzy > headroom {
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

func excitationOutcomeFromReading(reading excitationReading) ExcitationOutcome {
	outcome := ExcitationOutcome{
		Frenzy:             reading.frenzy,
		Saturation:         reading.saturation,
		Organic:            reading.organic,
		Exhaustion:         reading.exhaustion,
		Strength:           reading.strength,
		BranchingRatio:     reading.branchingRatio,
		StationarityMargin: reading.stationarityMargin,
		BaselineMu:         reading.baselineMu,
		IntensityRatio:     reading.intensityRatio,
		PoissonImprovement: reading.poissonImprovement,
		EventCount:         reading.eventCount,
		HighRisk:           reading.highRisk,
	}

	outcome.Eligible = excitationEligible(outcome)

	return outcome
}

func excitationEligible(outcome ExcitationOutcome) bool {
	if outcome.EventCount < 4 {
		return false
	}

	if !outcome.HighRisk {
		return true
	}

	if outcome.BranchingRatio >= 1 {
		return false
	}

	if outcome.StationarityMargin <= 0 {
		return false
	}

	return outcome.PoissonImprovement > 0
}

func excitationBatchBounds(batch []float64) (buyCount, sellCount, expectedLen int, ok bool) {
	if len(batch) < excitationPayloadHeader {
		return 0, 0, 0, false
	}

	buyValue := batch[2]
	sellValue := batch[3]

	if buyValue < 0 || sellValue < 0 ||
		buyValue != float64(int(buyValue)) ||
		sellValue != float64(int(sellValue)) {
		return 0, 0, 0, false
	}

	buyCount = int(buyValue)
	sellCount = int(sellValue)
	available := len(batch) - excitationPayloadHeader

	if buyCount > available {
		return 0, 0, 0, false
	}

	remaining := available - buyCount

	if sellCount > remaining {
		return 0, 0, 0, false
	}

	expectedLen = excitationPayloadHeader + buyCount + sellCount

	return buyCount, sellCount, expectedLen, true
}

func secondsToTimes(seconds []float64) []time.Time {
	times := make([]time.Time, len(seconds))

	for index, second := range seconds {
		wholeSeconds := int64(second)
		nanoseconds := int64((second - float64(wholeSeconds)) * float64(time.Second))
		times[index] = time.Unix(wholeSeconds, nanoseconds)
	}

	return times
}

func DeriveFitCooldown(windowSpan time.Duration) time.Duration {
	if windowSpan <= 0 {
		return 0
	}

	return windowSpan * hawkesFitCooldownMult
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
