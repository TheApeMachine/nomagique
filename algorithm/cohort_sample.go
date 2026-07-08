package algorithm

import (
	"math"
	"sort"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
	"github.com/theapemachine/nomagique/utils"
	"gonum.org/v1/gonum/stat"
)

const (
	cohortMinimumWindow = 2
	cohortMinimumPeers  = 1
	cohortHistoryCap    = 128
)

/*
CohortSample turns symbol price streams into the feature batch Cohort expects.
The window is derived from observed per-symbol return history and shared peer depth.
*/
type CohortSample struct {
	config  CohortSampleConfig
	symbols map[string]*cohortSymbol
}

/*
CohortSampleConfig controls only structural history retention.
*/
type CohortSampleConfig struct {
	HistoryCap int
}

/*
CohortSampleInput is one observed ticker price.
*/
type CohortSampleInput struct {
	Symbol string
	Price  float64
	At     time.Time
}

type cohortSymbol struct {
	lastPrice float64
	prices    []float64
	returns   []float64
	times     []int64
}

/*
NewCohortSample returns a typed cohort encoder.
*/
func NewCohortSample(config ...CohortSampleConfig) *CohortSample {
	return &CohortSample{
		config:  cohortSampleConfig(config),
		symbols: map[string]*cohortSymbol{},
	}
}

func cohortSampleConfig(config []CohortSampleConfig) CohortSampleConfig {
	if len(config) == 0 {
		return CohortSampleConfig{HistoryCap: cohortHistoryCap}
	}

	if config[0].HistoryCap < cohortMinimumWindow {
		config[0].HistoryCap = cohortHistoryCap
	}

	return config[0]
}

/*
Measure observes a tick and emits a feature frame once peer history is ready.
*/
func (cohortSample *CohortSample) Measure(
	input CohortSampleInput,
) (equation.FeatureFrame, bool, error) {
	sample, err := cohortSample.sample(input)
	if err != nil {
		return equation.FeatureFrame{}, false, err
	}

	window := cohortSample.window(sample.name)

	if window < cohortMinimumWindow {
		return equation.FeatureFrame{}, false, nil
	}

	pairCorrelations, peerCorrelations, peerEnergies, energy, barSpacingSeconds :=
		cohortSample.peers(sample.name, window)

	if len(pairCorrelations) == 0 ||
		len(peerCorrelations) < cohortMinimumPeers ||
		len(peerEnergies) < cohortMinimumPeers ||
		energy <= 0 ||
		barSpacingSeconds <= 0 {
		return equation.FeatureFrame{}, false, nil
	}

	features := cohortFeatures(
		window,
		barSpacingSeconds,
		energy,
		pairCorrelations,
		peerCorrelations,
		peerEnergies,
	)

	return equation.NewFeatureFrame(equation.CohortInputKeys, features), true, nil
}

type cohortTick struct {
	name  string
	price float64
	at    int64
}

func (cohortSample *CohortSample) sample(input CohortSampleInput) (cohortTick, error) {
	tick := cohortTick{
		name:  input.Symbol,
		price: input.Price,
		at:    input.At.UnixNano(),
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

	historyCap := cohortSample.historyCap()

	if len(symbolState.times) > 0 && symbolState.times[len(symbolState.times)-1] == tick.at {
		symbolState.prices[len(symbolState.prices)-1] = tick.price
		symbolState.lastPrice = tick.price

		return
	}

	symbolState.prices = utils.AppendRingFloat(symbolState.prices, tick.price, historyCap)
	symbolState.times = appendRingInt64(symbolState.times, tick.at, historyCap)

	if symbolState.lastPrice > 0 {
		symbolState.returns = utils.AppendRingFloat(
			symbolState.returns,
			math.Log(tick.price/symbolState.lastPrice),
			historyCap,
		)
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

func (cohortSample *CohortSample) historyCap() int {
	if cohortSample.config.HistoryCap < cohortMinimumWindow {
		return cohortHistoryCap
	}

	return cohortSample.config.HistoryCap
}

func (cohortSample *CohortSample) window(symbol string) int {
	symbolState := cohortSample.symbols[symbol]

	if symbolState == nil {
		return 0
	}

	if len(symbolState.returns) < cohortMinimumWindow {
		return 0
	}

	shortWindow, _, err := statistic.ResolveWindows(symbolState.returns, 0, 0)

	if err != nil {
		return 0
	}

	commonDepth := cohortSample.commonDepth()

	if commonDepth <= 0 {
		return 0
	}

	if shortWindow < cohortMinimumWindow {
		shortWindow = cohortMinimumWindow
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

func (cohortSample *CohortSample) peers(
	target string,
	window int,
) ([]float64, []float64, []float64, float64, float64) {
	gridInterval := cohortSample.gridInterval()

	if gridInterval <= 0 {
		return nil, nil, nil, 0, 0
	}

	currentTime := cohortSample.currentTime()
	windowStart := currentTime - int64(window)*gridInterval
	targetState := cohortSample.symbols[target]

	if targetState == nil {
		return nil, nil, nil, 0, 0
	}

	targetIntervals := returnIntervalsCohort(targetState, windowStart, currentTime)
	energy := medianAbsoluteIntervalsCohort(targetIntervals)

	if energy <= 0 {
		return nil, nil, nil, 0, 0
	}

	pairCorrelations := make([]float64, 0, len(cohortSample.symbols)-1)
	peerCorrelations := make([]float64, 0, len(cohortSample.symbols))
	peerEnergies := make([]float64, 0, len(cohortSample.symbols))
	symbols := cohortSample.sortedSymbols()

	for _, symbol := range symbols {
		symbolState := cohortSample.symbols[symbol]
		intervals := returnIntervalsCohort(symbolState, windowStart, currentTime)
		symbolEnergy := medianAbsoluteIntervalsCohort(intervals)

		if symbolEnergy > 0 {
			peerEnergies = append(peerEnergies, symbolEnergy)
		}

		if symbol == target {
			continue
		}

		if correlation, ok := pairCorrelationCohort(targetState, symbolState, gridInterval, windowStart, currentTime, window); ok {
			pairCorrelations = append(pairCorrelations, correlation)
		}
	}

	for leftIndex := 0; leftIndex < len(symbols); leftIndex++ {
		left := cohortSample.symbols[symbols[leftIndex]]

		for rightIndex := leftIndex + 1; rightIndex < len(symbols); rightIndex++ {
			right := cohortSample.symbols[symbols[rightIndex]]
			correlation, ok := pairCorrelationCohort(left, right, gridInterval, windowStart, currentTime, window)

			if ok {
				peerCorrelations = append(peerCorrelations, correlation)
			}
		}
	}

	return pairCorrelations,
		peerCorrelations,
		peerEnergies,
		energy,
		float64(gridInterval) / float64(time.Second)
}

func (cohortSample *CohortSample) gridInterval() int64 {
	deltas := make([]float64, 0, len(cohortSample.symbols))

	for _, symbolState := range cohortSample.symbols {
		for index := 1; index < len(symbolState.times); index++ {
			delta := symbolState.times[index] - symbolState.times[index-1]

			if delta > 0 {
				deltas = append(deltas, float64(delta))
			}
		}
	}

	median, ok := statistic.MedianOf(deltas)

	if !ok || median <= 0 || math.IsNaN(median) || math.IsInf(median, 0) {
		return 0
	}

	return int64(median)
}

func (cohortSample *CohortSample) sortedSymbols() []string {
	symbols := make([]string, 0, len(cohortSample.symbols))

	for symbol := range cohortSample.symbols {
		symbols = append(symbols, symbol)
	}

	sort.Strings(symbols)

	return symbols
}

func (cohortSample *CohortSample) currentTime() int64 {
	current := int64(0)

	for _, symbolState := range cohortSample.symbols {
		if len(symbolState.times) == 0 {
			continue
		}

		last := symbolState.times[len(symbolState.times)-1]

		if last > current {
			current = last
		}
	}

	return current
}

func pairCorrelationCohort(
	left *cohortSymbol,
	right *cohortSymbol,
	gridInterval int64,
	windowStart int64,
	currentTime int64,
	window int,
) (float64, bool) {
	leftGrid := gridReturnsCohort(left, gridInterval, windowStart, window)
	rightGrid := gridReturnsCohort(right, gridInterval, windowStart, window)

	if len(leftGrid) == window && len(rightGrid) == window {
		if correlation, ok := pearsonCohort(leftGrid, rightGrid); ok {
			return correlation, true
		}
	}

	return hayashiYoshidaCohort(
		returnIntervalsCohort(left, windowStart, currentTime),
		returnIntervalsCohort(right, windowStart, currentTime),
	)
}

func gridReturnsCohort(
	symbolState *cohortSymbol,
	gridInterval int64,
	windowStart int64,
	window int,
) []float64 {
	if symbolState == nil ||
		len(symbolState.prices) == 0 ||
		gridInterval <= 0 ||
		window < cohortMinimumWindow {
		return nil
	}

	prices := make([]float64, 0, window+1)

	for index := 0; index <= window; index++ {
		cutoff := windowStart + int64(index)*gridInterval
		price, ok := priceAtCohort(symbolState, cutoff)

		if !ok {
			return nil
		}

		prices = append(prices, price)
	}

	return logReturnsCohort(prices)
}

func pearsonCohort(left, right []float64) (float64, bool) {
	if len(left) != len(right) || len(left) < cohortMinimumWindow {
		return 0, false
	}

	meanLeft := stat.Mean(left, nil)
	meanRight := stat.Mean(right, nil)
	var leftVariance float64
	var rightVariance float64
	var covariance float64

	for index := range left {
		leftDelta := left[index] - meanLeft
		rightDelta := right[index] - meanRight

		leftVariance += leftDelta * leftDelta
		rightVariance += rightDelta * rightDelta
		covariance += leftDelta * rightDelta
	}

	if leftVariance <= 0 || rightVariance <= 0 {
		return 0, false
	}

	correlation := covariance / math.Sqrt(leftVariance*rightVariance)

	if math.IsNaN(correlation) || math.IsInf(correlation, 0) {
		return 0, false
	}

	return math.Max(-1, math.Min(1, correlation)), true
}

func priceAtCohort(symbolState *cohortSymbol, timestamp int64) (float64, bool) {
	for index, observed := range symbolState.times {
		if observed == timestamp && index < len(symbolState.prices) {
			price := symbolState.prices[index]

			return price, price > 0
		}
	}

	return 0, false
}

func cohortFeatures(
	window int,
	barSpacingSeconds float64,
	energy float64,
	pairCorrelations, peerCorrelations, peerEnergies []float64,
) []float64 {
	features := []float64{
		float64(window),
		float64(len(pairCorrelations)),
		float64(len(peerCorrelations)),
		float64(len(peerEnergies)),
		barSpacingSeconds,
		energy,
	}

	features = append(features, pairCorrelations...)
	features = append(features, peerCorrelations...)
	features = append(features, peerEnergies...)

	return features
}

func appendRingInt64(values []int64, value int64, capacity int) []int64 {
	values = append(values, value)

	if len(values) <= capacity {
		return values
	}

	return values[len(values)-capacity:]
}

func logReturnsCohort(prices []float64) []float64 {
	if len(prices) < 2 {
		return nil
	}

	returns := make([]float64, 0, len(prices)-1)

	for index := 1; index < len(prices); index++ {
		if prices[index-1] <= 0 || prices[index] <= 0 {
			continue
		}

		returns = append(returns, math.Log(prices[index]/prices[index-1]))
	}

	return returns
}

type cohortReturnInterval struct {
	start int64
	end   int64
	value float64
}

func returnIntervalsCohort(
	symbolState *cohortSymbol,
	windowStart int64,
	currentTime int64,
) []cohortReturnInterval {
	if symbolState == nil || len(symbolState.times) < 2 {
		return nil
	}

	intervals := make([]cohortReturnInterval, 0, len(symbolState.times)-1)

	for index := 1; index < len(symbolState.times); index++ {
		start := symbolState.times[index-1]
		end := symbolState.times[index]

		if end <= windowStart || start >= currentTime || end <= start {
			continue
		}

		previous := symbolState.prices[index-1]
		current := symbolState.prices[index]

		if previous <= 0 || current <= 0 {
			continue
		}

		intervals = append(intervals, cohortReturnInterval{
			start: start,
			end:   end,
			value: math.Log(current / previous),
		})
	}

	return intervals
}

func hayashiYoshidaCohort(
	left []cohortReturnInterval,
	right []cohortReturnInterval,
) (float64, bool) {
	if len(left) == 0 || len(right) == 0 {
		return 0, false
	}

	covariance := 0.0
	leftVariance := 0.0
	rightVariance := 0.0

	for _, interval := range left {
		leftVariance += interval.value * interval.value
	}

	for _, interval := range right {
		rightVariance += interval.value * interval.value
	}

	if leftVariance <= 0 || rightVariance <= 0 {
		return 0, false
	}

	for _, leftInterval := range left {
		for _, rightInterval := range right {
			if leftInterval.start < rightInterval.end &&
				rightInterval.start < leftInterval.end {
				covariance += leftInterval.value * rightInterval.value
			}
		}
	}

	correlation := covariance / math.Sqrt(leftVariance*rightVariance)

	if math.IsNaN(correlation) || math.IsInf(correlation, 0) {
		return 0, false
	}

	return math.Max(-1, math.Min(1, correlation)), true
}

func medianAbsoluteIntervalsCohort(intervals []cohortReturnInterval) float64 {
	absValues := make([]float64, 0, len(intervals))

	for _, interval := range intervals {
		absValues = append(absValues, math.Abs(interval.value))
	}

	median, ok := statistic.MedianOf(absValues)

	if !ok {
		return 0
	}

	return median
}
