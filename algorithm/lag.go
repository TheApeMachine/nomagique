package algorithm

import (
	"math"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/correlation"
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
LagInput carries typed lead-lag evidence.
*/
type LagInput struct {
	IsAnchor    bool
	Price       float64
	MoveReady   bool
	MoveMoved   bool
	StallMargin float64
	LagOK       bool
	LagBars     int
	LagCorr     float64
	ContempOK   bool
	ContempCorr float64
	SampleCount int
}

/*
Lag classifies anchor stall, inefficient lag, synchronized drift, and decoupling.
*/
type Lag struct {
	outcome LagOutcome
}

/*
NewLag returns a typed lead-lag classifier.
*/
func NewLag() *Lag {
	return &Lag{}
}

/*
LagInputFromFields decodes the legacy ordered feature vector into typed input.
*/
func LagInputFromFields(fields []float64) (LagInput, error) {
	if len(fields) < lagPayloadFields {
		return LagInput{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag: insufficient feature fields",
			nil,
		))
	}

	return LagInput{
		IsAnchor:    fields[0] > 0,
		Price:       fields[1],
		MoveReady:   fields[2] > 0,
		MoveMoved:   fields[3] > 0,
		StallMargin: fields[4],
		LagOK:       fields[5] > 0,
		LagBars:     int(fields[6]),
		LagCorr:     fields[7],
		ContempOK:   fields[8] > 0,
		ContempCorr: fields[9],
		SampleCount: int(fields[10]),
	}, nil
}

/*
Measure classifies typed lead-lag evidence.
*/
func (lag *Lag) Measure(input LagInput) (LagOutcome, error) {
	lag.outcome = lag.evaluate(input)

	if !lag.outcome.Eligible || lag.outcome.Strength <= 0 {
		return LagOutcome{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag: insufficient signal eligibility",
			nil,
		))
	}

	return lag.outcome, nil
}

func (lag *Lag) evaluate(input LagInput) LagOutcome {
	if input.Price <= 0 {
		return LagOutcome{}
	}

	if input.IsAnchor {
		return lag.evaluateAnchor(input.Price, input.MoveReady, input.MoveMoved, input.StallMargin)
	}

	return lag.evaluateFollower(
		input.Price,
		input.MoveReady,
		input.MoveMoved,
		input.StallMargin,
		input.LagOK,
		input.LagBars,
		input.LagCorr,
		input.ContempOK,
		input.ContempCorr,
		input.SampleCount,
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
	lag     *Lag
	project func(LagOutcome) float64
}

func newLagReading(lag *Lag, project func(LagOutcome) float64) *LagReading {
	return &LagReading{
		lag:     lag,
		project: project,
	}
}

/*
Measure projects one score from the last lag outcome.
*/
func (reading *LagReading) Measure() (float64, error) {
	if reading.lag == nil || reading.project == nil {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag: reading is incomplete",
			nil,
		))
	}

	return reading.project(reading.lag.outcome), nil
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
