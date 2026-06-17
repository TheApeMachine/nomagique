package algorithm

import (
	"math"
	"sync"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/hawkes"
	"github.com/theapemachine/nomagique/probability"
)

const (
	excitationPayloadHeader = 4
	bivariateParamCount     = 7
	hawkesGateHistoryCap    = 64
	hawkesFitCooldownMult   = 50
	uniformExcitationShare  = 1.0 / 4.0
)

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
	PoissonImprovement float64
	EventCount         int
	HighRisk           bool
	Eligible           bool
}

/*
Excitation fits a bivariate Hawkes process over buy/sell arrival times.

Payload layout: horizonSeconds, fitCooldownSeconds, buyCount, sellCount,
then buy arrival seconds, then sell arrival seconds.
Per-scope fit state is keyed from the artifact scope attribute.
*/
type Excitation struct {
	artifact *datura.Artifact
	symbols  sync.Map
	outcome  ExcitationOutcome
}

/*
NewExcitation returns a Hawkes excitation stage for io.ReadWriter pipelines.
*/
func NewExcitation() *Excitation {
	return &Excitation{
		artifact: datura.Acquire("excitation", datura.Artifact_Type_json),
	}
}

func (excitation *Excitation) Write(p []byte) (int, error) {
	return excitation.artifact.Write(p)
}

func (excitation *Excitation) Read(p []byte) (int, error) {
	scope, _ := excitation.artifact.Scope()

	rehydrateArtifact(&excitation.artifact, "excitation", datura.Artifact_Type_json)

	if scope == "" {
		scope, _ = excitation.artifact.Scope()
	}

	if scope == "" {
		scope = excitation.artifact.Peek("scope")
	}

	payload, payloadOK := excitation.artifact.PayloadQuiet()

	if payloadOK {
		excitation.outcome = excitation.evaluate(scope, payloadSamples(payload))
		excitation.publishReadings()
	}

	return excitation.artifact.Read(p)
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

func (excitation *Excitation) evaluate(scope string, batch []float64) ExcitationOutcome {
	if scope == "" || len(batch) < excitationPayloadHeader {
		return ExcitationOutcome{}
	}

	horizon := time.Unix(0, int64(batch[0]*float64(time.Second)))
	fitCooldown := time.Duration(batch[1] * float64(time.Second))
	buyCount := int(batch[2])
	sellCount := int(batch[3])

	if buyCount < 0 || sellCount < 0 {
		return ExcitationOutcome{}
	}

	expectedLen := excitationPayloadHeader + buyCount + sellCount

	if len(batch) < expectedLen {
		return ExcitationOutcome{}
	}

	buyTimes := secondsToTimes(batch[excitationPayloadHeader : excitationPayloadHeader+buyCount])
	sellTimes := secondsToTimes(batch[excitationPayloadHeader+buyCount : expectedLen])

	state := excitation.loadSymbol(scope)
	reading, ok := state.measure(buyTimes, sellTimes, horizon, fitCooldown)

	if !ok {
		return ExcitationOutcome{}
	}

	return excitationOutcomeFromReading(reading)
}

func (excitation *Excitation) publishReadings() {
	pokeFloat(excitation.artifact, "excitation.frenzy", excitation.outcome.Frenzy)
	pokeFloat(excitation.artifact, "excitation.saturation", excitation.outcome.Saturation)
	pokeFloat(excitation.artifact, "excitation.organic", excitation.outcome.Organic)
	pokeFloat(excitation.artifact, "excitation.exhaustion", excitation.outcome.Exhaustion)
	pokeFloat(excitation.artifact, "excitation.strength", excitation.outcome.Strength)
	pokeFloat(excitation.artifact, "excitation.branching_ratio", excitation.outcome.BranchingRatio)
	pokeFloat(excitation.artifact, "excitation.stationarity_margin", excitation.outcome.StationarityMargin)
	pokeFloat(excitation.artifact, "excitation.poisson_improvement", excitation.outcome.PoissonImprovement)
	pokeFloat(excitation.artifact, "excitation.event_count", float64(excitation.outcome.EventCount))

	if excitation.outcome.Eligible {
		pokeFloat(excitation.artifact, "excitation.eligible", 1)
	}
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
	if len(p) == 0 {
		return 0, nil
	}

	return len(p), nil
}

func (reading *ExcitationReading) Read(p []byte) (int, error) {
	value := 0.0

	if reading.excitation != nil && reading.project != nil {
		value = reading.project(reading.excitation.outcome)
	}

	_ = reading.artifact.SetPayload(encodePayload(value))

	return reading.artifact.Read(p)
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
	poissonImprovement float64
	eventCount         int
	highRisk           bool
}

type excitationSymbol struct {
	fit             hawkes.BivariateFit
	hasFit          bool
	lastFitEventKey fitEventKey
	lastFitTime     time.Time
	fitCooldown     time.Duration
	minFitEvents    int
	rawBase         *adaptive.EMA
	lastRawNorm     float64
	lastCategory    hawkes.FitCategory
	spectralRadii   []float64
	asymmetries     []float64
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
		rawBase:      adaptive.NewEMA(),
	}
}

func (symbol *excitationSymbol) measure(
	buyTimes, sellTimes []time.Time,
	horizon time.Time,
	fitCooldown time.Duration,
) (excitationReading, bool) {
	symbol.fitCooldown = fitCooldown

	stream := hawkes.NewArrivalStream(buyTimes, sellTimes)
	context, adaptiveStream, ok := fitContextFromStream(stream, horizon)

	if !ok || !context.EnoughEvents(adaptiveStream) {
		if !symbol.hasFit {
			return excitationReading{}, false
		}

		fallbackStream := hawkes.NewArrivalStream(buyTimes, sellTimes)

		if len(fallbackStream.BuyTimes())+len(fallbackStream.SellTimes()) == 0 {
			return excitationReading{}, false
		}

		fallbackFit := symbol.fit.WithIntensitiesAt(fallbackStream, horizon)
		reading, readingOk := symbol.measureFit(fallbackFit)

		return symbol.enrichReading(reading, readingOk, fallbackFit, fallbackStream, horizon)
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
		return excitationReading{
			strength:           raw,
			organic:            math.Max(raw, uniformExcitationShare),
			branchingRatio:     fit.SpectralRadius,
			stationarityMargin: 1 - fit.SpectralRadius,
		}, true
	}

	category, confidence := hawkes.ClassifyFit(fit, asymmetry, sellSide, gates)
	frenzy, saturation, organic, exhaustion := excitationTransitionScores(
		fit, asymmetry, sellSide, category, confidence, gates,
	)

	rawNorm := symbol.lastRawNorm
	symbol.lastRawNorm = symbol.rawBaseStep(raw)

	if rawNorm > 0 {
		saturationEvidence := competitionMargin(raw-rawNorm, rawNorm) * (1 - asymmetry)

		if saturationEvidence > confidence {
			category = hawkes.FitCategorySaturation
			confidence = saturationEvidence
			saturation = saturationEvidence
		}
	}

	if symbol.lastCategory == hawkes.FitCategoryFrenzy ||
		symbol.lastCategory == hawkes.FitCategorySaturation {
		exhaustionEvidence := competitionMargin(rawNorm-raw, rawNorm)

		if exhaustionEvidence > confidence {
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
		highRisk: category == hawkes.FitCategoryFrenzy ||
			category == hawkes.FitCategorySaturation,
	}, true
}

func (symbol *excitationSymbol) rawBaseStep(sample float64) float64 {
	inbound := datura.Acquire("excitation-ema-in", datura.Artifact_Type_json)
	_ = inbound.SetPayload(encodePayload(sample))
	frame, err := inbound.Message().Marshal()

	if err != nil {
		return sample
	}

	_, _ = symbol.rawBase.Write(frame)
	out := make([]byte, 4096)
	readCount, _ := symbol.rawBase.Read(out)
	outbound := datura.Acquire("excitation-ema-out", datura.Artifact_Type_json)
	_, _ = outbound.Write(out[:readCount])
	payload, payloadErr := outbound.Payload()

	if payloadErr != nil {
		return sample
	}

	value, ok := payloadScalar(payload)

	if !ok {
		return sample
	}

	return value
}

func (symbol *excitationSymbol) recordFitGates(spectralRadius, asymmetry float64) {
	if spectralRadius <= 0 {
		return
	}

	if math.IsNaN(asymmetry) || math.IsInf(asymmetry, 0) || asymmetry < 0 || asymmetry > 1 {
		return
	}

	symbol.spectralRadii = appendRingFloat(symbol.spectralRadii, spectralRadius, hawkesGateHistoryCap)
	symbol.asymmetries = appendRingFloat(symbol.asymmetries, asymmetry, hawkesGateHistoryCap)
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
		saturation = competitionMargin(margin, saturationRadius)

		if saturation > headroom {
			headroom = saturation
		}
	}

	if baseline > 0 && intensity >= baseline {
		margin := intensity - baseline
		organic = margin / (margin + baseline)

		if organic > headroom {
			headroom = organic
		}
	}

	if asymmetry < frenzyAsymmetry {
		margin := frenzyAsymmetry - asymmetry
		frenzy = competitionMargin(margin, frenzyAsymmetry)

		if frenzy > headroom {
			headroom = frenzy
		}
	}

	if headroom < 0 {
		headroom = uniformExcitationShare
	}

	return frenzy, saturation, organic, exhaustion
}

func competitionMargin(excess, span float64) float64 {
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
