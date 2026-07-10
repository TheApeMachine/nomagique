package algorithm

import (
	"sync"
	"time"

	"github.com/theapemachine/errnie"
)

const hawkesFitCooldownMult = 50

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
	SpectralRadius     float64
	StationarityMargin float64
	BaselineMu         float64
	IntensityRatio     float64
	PoissonImprovement float64
	EventCount         int
	HighRisk           bool
	Eligible           bool
	Maturity           float64
}

/*
Excitation fits a bivariate Hawkes process over buy/sell arrival times.
*/
type Excitation struct {
	symbols sync.Map
}

/*
NewExcitation returns a direct Hawkes excitation calculator.
*/
func NewExcitation() *Excitation {
	return &Excitation{}
}

/*
Measure scores one excitation batch.
*/
func (excitation *Excitation) Measure(
	input ExcitationInput,
) (ExcitationOutcome, bool, error) {
	if input.Symbol == "" || len(input.BuySeconds)+len(input.SellSeconds) == 0 {
		return ExcitationOutcome{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"excitation: invalid batch",
			nil,
		))
	}

	horizon := time.Unix(0, int64(input.HorizonSeconds*float64(time.Second)))
	fitCooldown := time.Duration(input.FitCooldownSeconds * float64(time.Second))
	buyTimes := secondsToTimes(input.BuySeconds)
	sellTimes := secondsToTimes(input.SellSeconds)
	symbolState := excitation.loadSymbol(input.Symbol)
	reading, ok := symbolState.measure(
		buyTimes, sellTimes, horizon, fitCooldown, input.TouchImbalance,
	)

	if !ok {
		return ExcitationOutcome{}, false, nil
	}

	return excitationOutcomeFromReading(reading), true, nil
}

func (excitation *Excitation) loadSymbol(scope string) *excitationSymbol {
	value, _ := excitation.symbols.LoadOrStore(scope, newExcitationSymbol())

	return value.(*excitationSymbol)
}

func excitationOutcomeFromReading(reading excitationReading) ExcitationOutcome {
	outcome := ExcitationOutcome{
		Frenzy:             reading.frenzy,
		Saturation:         reading.saturation,
		Organic:            reading.organic,
		Exhaustion:         reading.exhaustion,
		Strength:           reading.strength,
		BranchingRatio:     reading.branchingRatio,
		SpectralRadius:     reading.spectralRadius,
		StationarityMargin: reading.stationarityMargin,
		BaselineMu:         reading.baselineMu,
		IntensityRatio:     reading.intensityRatio,
		PoissonImprovement: reading.poissonImprovement,
		EventCount:         reading.eventCount,
		HighRisk:           reading.highRisk,
		Maturity:           reading.maturity,
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

	if outcome.SpectralRadius >= 1 {
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
