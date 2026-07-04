package algorithm

import (
	"io"
	"sync"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
)

const (
	excitationPayloadHeader = 5
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
	SpectralRadius     float64
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
	packed   []byte
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
	excitation.packed = nil
	excitation.artifact.WithPayload(payload)
	return len(payload), nil
}

func (excitation *Excitation) Read(payload []byte) (int, error) {
	if len(excitation.packed) == 0 {
		state := datura.Acquire("excitation-state", datura.APPJSON)

		if _, err := state.Unpack(excitation.artifact.DecryptPayload()); err != nil {
			state.Release()

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"excitation: state write failed",
				err,
			))
		}

		scope, scopeErr := state.Scope()

		if scopeErr != nil || scope == "" {
			state.Release()

			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"excitation: scope required",
				scopeErr,
			))
		}

		outcome, err := excitation.evaluateState(state, scope)

		if err != nil {
			state.Release()

			return 0, err
		}

		excitation.outcome = outcome

		state.MergeOutput("frenzy", excitation.outcome.Frenzy)
		state.MergeOutput("saturation", excitation.outcome.Saturation)
		state.MergeOutput("organic", excitation.outcome.Organic)
		state.MergeOutput("exhaustion", excitation.outcome.Exhaustion)
		state.MergeOutput("strength", excitation.outcome.Strength)
		state.MergeOutput("branchingRatio", excitation.outcome.BranchingRatio)
		state.MergeOutput("spectralRadius", excitation.outcome.SpectralRadius)
		state.MergeOutput("stationarityMargin", excitation.outcome.StationarityMargin)
		state.MergeOutput("baselineMu", excitation.outcome.BaselineMu)
		state.MergeOutput("intensityRatio", excitation.outcome.IntensityRatio)
		state.MergeOutput("ready", true)

		if excitation.outcome.Eligible {
			state.Merge("excitation.eligible", 1.0)
		}

		state.Poke("output", "root")
		state.Poke([]string{"frenzy", "saturation", "organic", "exhaustion", "strength"}, "inputs")

		packed, packErr := state.Message().MarshalPacked()
		state.Release()

		if packErr != nil {
			return 0, errnie.Error(errnie.Err(
				errnie.IO,
				"excitation: marshal packed failed",
				packErr,
			))
		}

		excitation.packed = packed
	}

	if len(excitation.packed) == 0 {
		return 0, io.EOF
	}

	readCount := copy(payload, excitation.packed)
	excitation.packed = excitation.packed[readCount:]

	if len(excitation.packed) == 0 {
		return readCount, io.EOF
	}

	return readCount, nil
}

func (excitation *Excitation) Close() error {
	return nil
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
