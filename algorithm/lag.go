package algorithm

import (
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/correlation"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

const lagPayloadFields = 11

/*
LagOutcome holds lead-lag classification scores.
*/
type LagOutcome struct {
	InefficientScore float64
	SyncScore        float64
	DecoupledScore   float64
	StallScore       float64
	Strength         float64
	Category         int
	Eligible         bool
	Price            float64
}

/*
Lag classifies anchor stall, inefficient lag, synchronized drift, and decoupling.

Payload layout: isAnchor, price, moveReady, moveMoved, stallMargin, lagOK, lagBars,
lagCorr, contempOK, contempCorr, sampleCount.
*/
type Lag struct {
	artifact *datura.Artifact
	outcome  LagOutcome
}

/*
NewLag returns a lead-lag classification stage wired from config on the artifact.
*/
func NewLag(artifact *datura.Artifact) *Lag {
	return &Lag{
		artifact: artifact,
	}
}

func (lag *Lag) Write(payload []byte) (int, error) {
	lag.artifact.WithPayload(payload)
	return len(payload), nil
}

func (lag *Lag) Read(payload []byte) (int, error) {
	state := datura.Acquire("lag-state", datura.APPJSON)

	if _, err := state.Unpack(lag.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag: state write failed",
			err,
		))
	}

	inputKeys := equation.EnsureFeatureSchema(state, lag.artifact, equation.LagInputKeys)
	fields, err := equation.FeatureFields(state, inputKeys)

	if err != nil {
		return 0, err
	}

	if len(fields) < lagPayloadFields {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag: insufficient feature fields",
			nil,
		))
	}

	lag.outcome = lag.evaluateFields(fields)

	if !lag.outcome.Eligible || lag.outcome.Strength <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag: insufficient signal eligibility",
			nil,
		))
	}

	state.MergeOutput("inefficient", lag.outcome.InefficientScore)
	state.MergeOutput("sync", lag.outcome.SyncScore)
	state.MergeOutput("decoupled", lag.outcome.DecoupledScore)
	state.MergeOutput("stall", lag.outcome.StallScore)
	state.MergeOutput("strength", lag.outcome.Strength)
	state.MergeOutput("value", float64(lag.outcome.Category))
	state.Poke("output", "root")
	state.Poke([]string{"inefficient", "sync", "decoupled", "stall", "strength"}, "inputs")

	return state.PackInto(payload)
}

func (lag *Lag) Close() error {
	return nil
}

func (lag *Lag) evaluateFields(fields []float64) LagOutcome {
	if len(fields) < lagPayloadFields {
		return LagOutcome{}
	}

	isAnchor := fields[0] > 0
	price := fields[1]
	moveReady := fields[2] > 0
	moveMoved := fields[3] > 0
	stallMargin := fields[4]
	lagOK := fields[5] > 0
	lagBars := int(fields[6])
	lagCorr := fields[7]
	contempOK := fields[8] > 0
	contempCorr := fields[9]
	sampleCount := int(fields[10])

	if price <= 0 {
		return LagOutcome{}
	}

	if isAnchor {
		return lag.evaluateAnchor(price, moveReady, moveMoved, stallMargin)
	}

	return lag.evaluateFollower(
		price,
		moveReady,
		moveMoved,
		stallMargin,
		lagOK,
		lagBars,
		lagCorr,
		contempOK,
		contempCorr,
		sampleCount,
	)
}

func (lag *Lag) evaluateAnchor(
	price float64,
	moveReady, moveMoved bool,
	stallMargin float64,
) LagOutcome {
	if !moveReady || moveMoved {
		return LagOutcome{}
	}

	strength := stallMargin
	stallScore := strength

	if strength <= 0 {
		return LagOutcome{}
	}

	return LagOutcome{
		StallScore: stallScore,
		Strength:   strength,
		Category:   4,
		Eligible:   true,
		Price:      price,
	}
}

func (lag *Lag) evaluateFollower(
	price float64,
	moveReady, moveMoved bool,
	stallMargin float64,
	lagOK bool,
	lagBars int,
	lagCorr float64,
	contempOK bool,
	contempCorr float64,
	sampleCount int,
) LagOutcome {
	if !moveReady {
		if contempOK {
			return lag.contemporaneousOutcome(price, contempCorr, sampleCount)
		}

		return LagOutcome{}
	}

	if !moveMoved {
		if contempOK {
			return lag.contemporaneousOutcome(price, contempCorr, sampleCount)
		}

		if stallMargin <= 0 {
			return LagOutcome{}
		}

		return LagOutcome{
			DecoupledScore: stallMargin,
			Strength:       stallMargin,
			Category:       3,
			Eligible:       true,
			Price:          price,
		}
	}

	if lagOK {
		outcome, err := lag.lagOutcome(price, lagBars, lagCorr)

		if err != nil {
			errnie.Error(errnie.Err(errnie.Validation, "lag: lag outcome failed", err))

			return LagOutcome{}
		}

		return outcome
	}

	if contempOK {
		return lag.contemporaneousOutcome(price, contempCorr, sampleCount)
	}

	return LagOutcome{}
}

func (lag *Lag) lagOutcome(price float64, lagBars int, corr float64) (LagOutcome, error) {
	maxBars, err := lagMaxBars()

	if err != nil {
		return LagOutcome{}, err
	}

	lagMagnitude := math.Abs(float64(lagBars))
	lagFraction := lagMagnitude / float64(maxBars)
	threshold := minLagFraction(maxBars)

	category := 2
	inefficientScore := 0.0
	syncScore := 0.0

	if lagFraction >= threshold {
		category = 1
		inefficientScore = lagFraction * corr
	}

	if category == 2 {
		syncScore = corr * (threshold - lagFraction)
	}

	strength := lagFraction

	return LagOutcome{
		InefficientScore: inefficientScore,
		SyncScore:        syncScore,
		Strength:         strength,
		Category:         category,
		Eligible:         true,
		Price:            price,
	}, nil
}

func (lag *Lag) contemporaneousOutcome(price, corr float64, sampleCount int) LagOutcome {
	if sampleCount <= 0 {
		return LagOutcome{}
	}

	significance := 1 / (2 * math.Sqrt(float64(sampleCount)))

	category := 3
	decoupledScore := math.Max(0, significance-corr)
	syncScore := 0.0

	if corr >= significance {
		category = 2
		syncScore = corr
		decoupledScore = 0
	}

	strength := decoupledScore

	if category == 2 {
		strength = syncScore
	}

	return LagOutcome{
		SyncScore:      syncScore,
		DecoupledScore: decoupledScore,
		Strength:       strength,
		Category:       category,
		Eligible:       true,
		Price:          price,
	}
}

func minLagFraction(maxBars int) float64 {
	if maxBars <= 0 {
		return 1
	}

	return math.Ceil(float64(maxBars)/2) / float64(maxBars)
}

func lagMinSamples() (int, error) {
	_, longWindow, err := statistic.ResolveWindows(make([]float64, 1), 0, 0)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag: min samples window resolution failed",
			err,
		))
	}

	return longWindow, nil
}

func lagMaxBars() (int, error) {
	_, longWindow, err := statistic.ResolveWindows(make([]float64, 1), 0, 0)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag: max bars window resolution failed",
			err,
		))
	}

	return longWindow, nil
}

func maxLagBarsForSeries(sampleCount int) (int, error) {
	if sampleCount <= 0 {
		return lagMaxBars()
	}

	_, longWindow, err := statistic.ResolveWindows(make([]float64, sampleCount), 0, 0)

	if err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag: series window resolution failed",
			err,
		))
	}

	halfSeries := sampleCount / 2

	if longWindow > halfSeries {
		longWindow = halfSeries
	}

	if longWindow < 1 {
		longWindow = 1
	}

	return longWindow, nil
}

func (lag *Lag) InefficientReading() *LagReading {
	return newLagReading(lag, func(outcome LagOutcome) float64 {
		return outcome.InefficientScore
	})
}

func (lag *Lag) SyncReading() *LagReading {
	return newLagReading(lag, func(outcome LagOutcome) float64 {
		return outcome.SyncScore
	})
}

func (lag *Lag) DecoupledReading() *LagReading {
	return newLagReading(lag, func(outcome LagOutcome) float64 {
		return outcome.DecoupledScore
	})
}

func (lag *Lag) StallReading() *LagReading {
	return newLagReading(lag, func(outcome LagOutcome) float64 {
		return outcome.StallScore
	})
}

type LagReading struct {
	artifact *datura.Artifact
	lag      *Lag
	project  func(LagOutcome) float64
}

func newLagReading(lag *Lag, project func(LagOutcome) float64) *LagReading {
	return &LagReading{
		artifact: datura.Acquire("lag-reading", datura.Artifact_Type_json),
		lag:      lag,
		project:  project,
	}
}

func (reading *LagReading) Write(p []byte) (int, error) {
	reading.artifact.WithPayload(p)
	return len(p), nil
}

func (reading *LagReading) Read(payload []byte) (int, error) {
	state := datura.Acquire("lag-reading-state", datura.APPJSON)

	if _, err := state.Unpack(reading.artifact.DecryptPayload()); err != nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag: state write failed",
			err,
		))
	}

	value := 0.0

	if reading.lag != nil && reading.project != nil {
		value = reading.project(reading.lag.outcome)
	}

	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

func (reading *LagReading) Close() error {
	return nil
}

/*
HayashiPairCorrelation computes asynchronous correlation between two price paths.
*/
func HayashiPairCorrelation(
	left, right []correlation.Sample,
	maxInterval time.Duration,
) (float64, bool) {
	return hayashiCorrelation(left, right, maxInterval)
}

/*
ShiftPriceSamples shifts sample timestamps by the given duration.
*/
func ShiftPriceSamples(
	samples []correlation.Sample,
	shift time.Duration,
) []correlation.Sample {
	if shift == 0 || len(samples) == 0 {
		return append([]correlation.Sample(nil), samples...)
	}

	shifted := make([]correlation.Sample, len(samples))

	for index, sample := range samples {
		shifted[index] = correlation.Sample{
			At:    sample.At.Add(shift),
			Value: sample.Value,
		}
	}

	return shifted
}

/*
CrossLagScore searches shifted anchor paths for the best Hayashi correlation.
*/
func CrossLagScore(
	anchorSeries, followerSeries []correlation.Sample,
	barInterval time.Duration,
) (lagBars int, corr float64, ok bool) {
	minSamples, err := lagMinSamples()

	if err != nil {
		errnie.Error(errnie.Err(errnie.Validation, "lag: cross lag min samples failed", err))

		return 0, 0, false
	}

	if len(anchorSeries) < minSamples || len(followerSeries) < minSamples {
		return 0, 0, false
	}

	sampleCount := len(anchorSeries)

	if len(followerSeries) < sampleCount {
		sampleCount = len(followerSeries)
	}

	maxLagBars, err := maxLagBarsForSeries(sampleCount)

	if err != nil {
		errnie.Error(errnie.Err(errnie.Validation, "lag: cross lag max bars failed", err))

		return 0, 0, false
	}

	baseline := 0.0

	if baselineCorr, baselineOK := hayashiCorrelation(anchorSeries, followerSeries, 0); baselineOK {
		baseline = baselineCorr
	}

	bestCorr := 0.0
	bestLag := 0

	for lag := -maxLagBars; lag <= maxLagBars; lag++ {
		if lag == 0 {
			continue
		}

		shifted := ShiftPriceSamples(anchorSeries, time.Duration(lag)*barInterval)
		lagCorr, lagOK := hayashiCorrelation(shifted, followerSeries, 0)

		if lagOK && lagCorr > bestCorr {
			bestCorr = lagCorr
			bestLag = lag
		}
	}

	significance := 1 / (2 * math.Sqrt(float64(sampleCount)))

	if bestLag == 0 || bestCorr <= significance {
		return 0, 0, false
	}

	floor := baseline

	if floor < 0 {
		floor = 0
	}

	margin := significance

	if relative := significance * math.Abs(baseline); relative > margin {
		margin = relative
	}

	if bestCorr <= floor+margin {
		return 0, 0, false
	}

	return bestLag, bestCorr, true
}
