package algorithm

import (
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
	"gonum.org/v1/gonum/stat"
)

const (
	cohortMinimumWindow = 2
	cohortMinimumPeers  = 2
)

/*
CohortSample turns symbol price streams into the feature batch Cohort expects.
The window is derived from observed per-symbol return history and shared peer depth.
*/
type CohortSample struct {
	artifact *datura.Artifact
	symbols  map[string]*cohortSymbol
}

type cohortSymbol struct {
	lastPrice float64
	returns   []float64
	times     []int64
}

/*
NewCohortSample returns a cohort encoder wired from config attributes.
*/
func NewCohortSample(artifact *datura.Artifact) *CohortSample {
	if artifact == nil {
		artifact = datura.Acquire("cohort-sample", datura.APPJSON)
	}

	return &CohortSample{
		artifact: artifact,
		symbols:  map[string]*cohortSymbol{},
	}
}

func (cohortSample *CohortSample) Write(payload []byte) (int, error) {
	cohortSample.artifact.WithPayload(payload)
	return len(payload), nil
}

func (cohortSample *CohortSample) Read(payload []byte) (int, error) {
	state := datura.Acquire("cohort-sample-state", datura.APPJSON)

	if _, err := state.Write(cohortSample.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	sample, sampleErr := cohortSample.sample(state)

	if sampleErr != nil {
		return 0, sampleErr
	}

	window := cohortSample.window(sample.name)

	if window < cohortMinimumWindow {
		return 0, io.EOF
	}

	symbolReturns := tailCohort(cohortSample.symbols[sample.name].returns, window)
	marketReturns, peerCorrelations, peerEnergies := cohortSample.peers(window)

	if len(symbolReturns) < window ||
		len(marketReturns) < window ||
		len(peerCorrelations) < cohortMinimumPeers {
		return 0, io.EOF
	}

	features := cohortFeatures(
		window,
		cohortSample.barSpacing(sample.name, window),
		symbolReturns,
		marketReturns,
		peerCorrelations,
		peerEnergies,
	)

	state.Merge("features", features)
	state.Poke("features", "root")
	state.Poke(equation.CohortInputKeys, "inputs")

	return state.Read(payload)
}

func (cohortSample *CohortSample) Close() error {
	return nil
}

type cohortTick struct {
	name  string
	price float64
	at    int64
}

func (cohortSample *CohortSample) sample(state *datura.Artifact) (cohortTick, error) {
	expectedChannel := datura.Peek[string](cohortSample.artifact, "channel")
	channel := datura.Peek[string](state, "channel")

	if expectedChannel != "" && channel != expectedChannel {
		return cohortTick{}, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("cohort-sample: expected %s channel, got %q", expectedChannel, channel),
			nil,
		))
	}

	root := datura.Peek[string](cohortSample.artifact, "root")
	symbolInput := datura.Peek[string](cohortSample.artifact, "symbolInput")
	priceInput := datura.Peek[string](cohortSample.artifact, "priceInput")

	if root == "" || symbolInput == "" || priceInput == "" {
		return cohortTick{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"cohort-sample: root, symbolInput, and priceInput are required",
			nil,
		))
	}

	tick := cohortTick{
		name:  datura.Peek[string](state, root, 0, symbolInput),
		price: datura.Peek[float64](state, root, 0, priceInput),
		at:    state.Timestamp(),
	}

	if tick.name == "" || tick.price <= 0 || tick.at <= 0 {
		return cohortTick{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"cohort-sample: frame requires symbol, positive price, and UnixNano timestamp",
			nil,
		))
	}

	if math.IsNaN(tick.price) || math.IsInf(tick.price, 0) {
		return cohortTick{}, errnie.Error(errnie.Err(
			errnie.Validation,
			"cohort-sample: price is non-finite",
			nil,
		))
	}

	cohortSample.observe(tick)

	return tick, nil
}

func (cohortSample *CohortSample) observe(tick cohortTick) {
	symbolState := cohortSample.symbol(tick.name)

	if symbolState.lastPrice > 0 {
		// ponytail: return history grows with runtime; use a time-bounded ring once retention policy exists.
		symbolState.returns = append(symbolState.returns, math.Log(tick.price/symbolState.lastPrice))
		symbolState.times = append(symbolState.times, tick.at)
	}

	symbolState.lastPrice = tick.price
}

func (cohortSample *CohortSample) symbol(name string) *cohortSymbol {
	existing, ok := cohortSample.symbols[name]

	if ok {
		return existing
	}

	symbolState := &cohortSymbol{}
	cohortSample.symbols[name] = symbolState

	return symbolState
}

func (cohortSample *CohortSample) window(symbol string) int {
	symbolState := cohortSample.symbols[symbol]

	if symbolState == nil {
		return 0
	}

	shortWindow, _, err := statistic.NewRollingWindow(0, 0).Resolve(symbolState.returns)

	if err != nil {
		return 0
	}

	commonDepth := cohortSample.commonDepth()

	if commonDepth <= 0 {
		return 0
	}

	if shortWindow > commonDepth {
		return commonDepth
	}

	return shortWindow
}

func (cohortSample *CohortSample) commonDepth() int {
	commonDepth := 0

	for _, symbolState := range cohortSample.symbols {
		if len(symbolState.returns) < cohortMinimumWindow {
			continue
		}

		if commonDepth == 0 || len(symbolState.returns) < commonDepth {
			commonDepth = len(symbolState.returns)
		}
	}

	return commonDepth
}

func (cohortSample *CohortSample) peers(window int) ([]float64, []float64, []float64) {
	series := make([][]float64, 0, len(cohortSample.symbols))

	for _, symbolState := range cohortSample.symbols {
		returns := tailCohort(symbolState.returns, window)

		if len(returns) < window {
			continue
		}

		series = append(series, returns)
	}

	if len(series) < cohortMinimumPeers {
		return nil, nil, nil
	}

	marketReturns := make([]float64, window)

	for index := range window {
		column := make([]float64, len(series))

		for seriesIndex, returns := range series {
			column[seriesIndex] = returns[index]
		}

		sort.Float64s(column)
		marketReturns[index] = stat.Quantile(0.5, stat.LinInterp, column, nil)
	}

	peerCorrelations := make([]float64, len(series))
	peerEnergies := make([]float64, len(series))

	for index, returns := range series {
		peerCorrelations[index] = stat.Correlation(returns, marketReturns, nil)
		peerEnergies[index] = medianAbsoluteCohort(returns)
	}

	return marketReturns, peerCorrelations, peerEnergies
}

func (cohortSample *CohortSample) barSpacing(symbol string, window int) float64 {
	symbolState := cohortSample.symbols[symbol]

	if symbolState == nil || len(symbolState.times) < cohortMinimumWindow {
		return 0
	}

	times := tailCohortInt(symbolState.times, window)
	deltas := make([]float64, 0, len(times))

	for index := 1; index < len(times); index++ {
		delta := times[index] - times[index-1]

		if delta <= 0 {
			continue
		}

		deltas = append(deltas, float64(delta)/float64(time.Second))
	}

	median, ok := statistic.MedianOf(deltas)

	if !ok {
		return 0
	}

	return median
}

func cohortFeatures(
	window int,
	barSpacingSeconds float64,
	symbolReturns, marketReturns, peerCorrelations, peerEnergies []float64,
) []float64 {
	features := []float64{
		float64(window),
		float64(len(symbolReturns)),
		float64(len(marketReturns)),
		float64(len(peerCorrelations)),
		float64(len(peerEnergies)),
		barSpacingSeconds,
	}

	features = append(features, symbolReturns...)
	features = append(features, marketReturns...)
	features = append(features, peerCorrelations...)
	features = append(features, peerEnergies...)

	return features
}

func tailCohort(values []float64, count int) []float64 {
	if len(values) == 0 || count <= 0 {
		return nil
	}

	if len(values) <= count {
		out := make([]float64, len(values))
		copy(out, values)

		return out
	}

	out := make([]float64, count)
	copy(out, values[len(values)-count:])

	return out
}

func tailCohortInt(values []int64, count int) []int64 {
	if len(values) == 0 || count <= 0 {
		return nil
	}

	if len(values) <= count {
		out := make([]int64, len(values))
		copy(out, values)

		return out
	}

	out := make([]int64, count)
	copy(out, values[len(values)-count:])

	return out
}

func medianAbsoluteCohort(values []float64) float64 {
	absValues := make([]float64, 0, len(values))

	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}

		absValues = append(absValues, math.Abs(value))
	}

	median, ok := statistic.MedianOf(absValues)

	if !ok {
		return 0
	}

	return median
}
