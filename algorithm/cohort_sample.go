package algorithm

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
	"github.com/theapemachine/nomagique/utils"
	"gonum.org/v1/gonum/stat"
)

/*
cohortMinimumPeers bounds sampling or gates so equation feature frames stay well-posed for symm envelope wiring.
*/
/*
cohortHistoryCap bounds sampling or gates so equation feature frames stay well-posed for symm envelope wiring.
*/
/*
cohortMinimumWindow bounds sampling or gates so equation feature frames stay well-posed for symm envelope wiring.
*/
const (
	cohortMinimumWindow = 2
	cohortMinimumPeers  = 1
	cohortHistoryCap    = 128
	// maxCohortPeers caps the all-pairs matrix. Full-board 605² peer math on
	// every disruptor tick freezes the signal ring; herd classification only
	// needs a deep peer sample.
	maxCohortPeers = 32
)

/*
CohortSample turns symbol price streams into the feature batch Cohort expects.
The window is derived from observed per-symbol return history and shared peer depth.
Measure locks the whole board: peer correlation reads every symbol's history.
*/
type CohortSample struct {
	mu      sync.Mutex
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

/*
cohortSymbol holds per-symbol sampling state that feeds equation Measure and Map outputs into resonance features.
*/
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

/*
cohortSampleConfig supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
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
	frames, err := cohortSample.MeasureMany([]CohortSampleInput{input})

	if err != nil {
		return equation.FeatureFrame{}, false, err
	}

	frame, ok := frames[input.Symbol]

	return frame, ok, nil
}

/*
ObserveMany records prices without building feature frames — heartbeat marks
must warm history without paying O(peers²) on the disruptor path.
*/
func (cohortSample *CohortSample) ObserveMany(inputs []CohortSampleInput) error {
	cohortSample.mu.Lock()
	defer cohortSample.mu.Unlock()

	return cohortSample.observeManyLocked(inputs)
}

/*
MeasureMany observes inputs then emits frames for those symbols.
*/
func (cohortSample *CohortSample) MeasureMany(
	inputs []CohortSampleInput,
) (map[string]equation.FeatureFrame, error) {
	cohortSample.mu.Lock()
	defer cohortSample.mu.Unlock()

	err := cohortSample.observeManyLocked(inputs)
	frames := cohortSample.emitManyLocked(inputSymbols(inputs))

	return frames, err
}

/*
EmitMany builds feature frames for named symbols that already have history.
Peer work is capped at maxCohortPeers and shared per window.
*/
func (cohortSample *CohortSample) EmitMany(
	names []string,
) (map[string]equation.FeatureFrame, error) {
	cohortSample.mu.Lock()
	defer cohortSample.mu.Unlock()

	return cohortSample.emitManyLocked(names), nil
}

func (cohortSample *CohortSample) observeManyLocked(inputs []CohortSampleInput) error {
	var firstErr error

	for _, input := range inputs {
		if _, err := cohortSample.sample(input); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (cohortSample *CohortSample) emitManyLocked(
	names []string,
) map[string]equation.FeatureFrame {
	out := make(map[string]equation.FeatureFrame, len(names))
	targets := uniqueNames(names)

	if len(targets) == 0 {
		return out
	}

	peers := cohortSample.peerUniverse(targets)
	gridInterval := cohortSample.gridIntervalAmong(peers)

	if gridInterval <= 0 {
		return out
	}

	currentTime := cohortSample.currentTimeAmong(peers)
	barSpacingSeconds := float64(gridInterval) / float64(time.Second)
	peerCache := make(map[int]cohortPeerBoard, 4)

	for _, name := range targets {
		window := cohortSample.window(name, peers)

		if window < cohortMinimumWindow {
			continue
		}

		board, ok := peerCache[window]

		if !ok {
			board = cohortSample.peerBoard(peers, gridInterval, currentTime, window)
			peerCache[window] = board
		}

		if len(board.peerCorrelations) < cohortMinimumPeers ||
			len(board.peerEnergies) < cohortMinimumPeers ||
			barSpacingSeconds <= 0 {
			continue
		}

		targetState := cohortSample.symbols[name]

		if targetState == nil {
			continue
		}

		windowStart := currentTime - int64(window)*gridInterval
		targetIntervals := returnIntervalsCohort(targetState, windowStart, currentTime)
		energy := medianAbsoluteIntervalsCohort(targetIntervals)

		if energy <= 0 {
			continue
		}

		pairCorrelations := make([]float64, 0, len(peers)-1)

		for _, symbol := range peers {
			if symbol == name {
				continue
			}

			symbolState := cohortSample.symbols[symbol]

			if correlation, ok := pairCorrelationCohort(
				targetState, symbolState, gridInterval, windowStart, currentTime, window,
			); ok {
				pairCorrelations = append(pairCorrelations, correlation)
			}
		}

		if len(pairCorrelations) == 0 {
			continue
		}

		out[name] = equation.NewFeatureFrame(
			equation.CohortInputKeys,
			cohortFeatures(
				window,
				barSpacingSeconds,
				energy,
				pairCorrelations,
				board.peerCorrelations,
				board.peerEnergies,
			),
		)
	}

	return out
}

func inputSymbols(inputs []CohortSampleInput) []string {
	names := make([]string, 0, len(inputs))

	for _, input := range inputs {
		if input.Symbol != "" {
			names = append(names, input.Symbol)
		}
	}

	return names
}

func uniqueNames(names []string) []string {
	out := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))

	for _, name := range names {
		if name == "" {
			continue
		}

		if _, ok := seen[name]; ok {
			continue
		}

		seen[name] = struct{}{}
		out = append(out, name)
	}

	return out
}

/*
peerUniverse returns at most maxCohortPeers symbols with enough history,
always including mustInclude, preferring the deepest return rings.
*/
func (cohortSample *CohortSample) peerUniverse(mustInclude []string) []string {
	type ranked struct {
		name  string
		depth int
	}

	must := make(map[string]struct{}, len(mustInclude))
	candidates := make([]ranked, 0, len(cohortSample.symbols))

	for _, name := range mustInclude {
		if name == "" {
			continue
		}

		must[name] = struct{}{}
	}

	for name, state := range cohortSample.symbols {
		if state == nil || len(state.returns) < cohortMinimumWindow {
			continue
		}

		candidates = append(candidates, ranked{name: name, depth: len(state.returns)})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].depth != candidates[j].depth {
			return candidates[i].depth > candidates[j].depth
		}

		return candidates[i].name < candidates[j].name
	})

	out := make([]string, 0, maxCohortPeers)
	picked := make(map[string]struct{}, maxCohortPeers)

	for name := range must {
		if state := cohortSample.symbols[name]; state == nil ||
			len(state.returns) < cohortMinimumWindow {
			continue
		}

		out = append(out, name)
		picked[name] = struct{}{}
	}

	for _, candidate := range candidates {
		if len(out) >= maxCohortPeers {
			break
		}

		if _, ok := picked[candidate.name]; ok {
			continue
		}

		out = append(out, candidate.name)
		picked[candidate.name] = struct{}{}
	}

	sort.Strings(out)

	return out
}

/*
cohortTick holds per-symbol sampling state that feeds equation Measure and Map outputs into resonance features.
*/
type cohortTick struct {
	name  string
	price float64
	at    int64
}

/*
(…).sample supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
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

/*
(…).observe supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
func (cohortSample *CohortSample) observe(tick cohortTick) {
	symbolState := cohortSample.symbol(tick.name)

	historyCap := cohortSample.historyCap()

	if n := len(symbolState.times); n > 0 {
		last := symbolState.times[n-1]

		if tick.at < last {
			return
		}

		if tick.at == last {
			symbolState.prices[len(symbolState.prices)-1] = tick.price
			symbolState.lastPrice = tick.price

			return
		}
	}

	symbolState.prices = utils.AppendRingFloat(symbolState.prices, tick.price, historyCap)
	symbolState.times = appendRingInt64(symbolState.times, tick.at, historyCap)

	if symbolState.lastPrice > 0 && tick.price > 0 {
		ratio := tick.price / symbolState.lastPrice

		if ratio > 0 {
			symbolState.returns = utils.AppendRingFloat(
				symbolState.returns,
				math.Log(ratio),
				historyCap,
			)
		}
	}

	symbolState.lastPrice = tick.price
}

/*
(…).symbol supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
func (cohortSample *CohortSample) symbol(name string) *cohortSymbol {
	existing, ok := cohortSample.symbols[name]

	if ok {
		return existing
	}

	symbolState := &cohortSymbol{}
	cohortSample.symbols[name] = symbolState

	return symbolState
}

/*
(…).historyCap supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
func (cohortSample *CohortSample) historyCap() int {
	if cohortSample.config.HistoryCap < cohortMinimumWindow {
		return cohortHistoryCap
	}

	return cohortSample.config.HistoryCap
}

/*
(…).window supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
func (cohortSample *CohortSample) window(symbol string, peers []string) int {
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

	commonDepth := cohortSample.commonDepth(peers)

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

/*
(…).commonDepth supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
func (cohortSample *CohortSample) commonDepth(peers []string) int {
	commonDepth := 0

	for _, name := range peers {
		symbolState := cohortSample.symbols[name]

		if symbolState == nil || len(symbolState.returns) < cohortMinimumWindow {
			continue
		}

		if commonDepth == 0 || len(symbolState.returns) < commonDepth {
			commonDepth = len(symbolState.returns)
		}
	}

	return commonDepth
}

/*
cohortPeerBoard is the window-scoped all-pairs matrix shared across targets in
one MeasureMany call.
*/
type cohortPeerBoard struct {
	peerCorrelations []float64
	peerEnergies     []float64
}

/*
peerBoard builds board-wide peer correlations and energies for one window.
*/
func (cohortSample *CohortSample) peerBoard(
	symbols []string,
	gridInterval, currentTime int64,
	window int,
) cohortPeerBoard {
	windowStart := currentTime - int64(window)*gridInterval
	peerCorrelations := make([]float64, 0, len(symbols))
	peerEnergies := make([]float64, 0, len(symbols))

	for _, symbol := range symbols {
		symbolState := cohortSample.symbols[symbol]
		intervals := returnIntervalsCohort(symbolState, windowStart, currentTime)
		symbolEnergy := medianAbsoluteIntervalsCohort(intervals)

		if symbolEnergy > 0 {
			peerEnergies = append(peerEnergies, symbolEnergy)
		}
	}

	for leftIndex := range symbols {
		left := cohortSample.symbols[symbols[leftIndex]]

		for rightIndex := leftIndex + 1; rightIndex < len(symbols); rightIndex++ {
			right := cohortSample.symbols[symbols[rightIndex]]
			correlation, ok := pairCorrelationCohort(
				left, right, gridInterval, windowStart, currentTime, window,
			)

			if ok {
				peerCorrelations = append(peerCorrelations, correlation)
			}
		}
	}

	return cohortPeerBoard{
		peerCorrelations: peerCorrelations,
		peerEnergies:     peerEnergies,
	}
}

/*
(…).gridInterval supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
func (cohortSample *CohortSample) gridInterval() int64 {
	return cohortSample.gridIntervalAmong(cohortSample.sortedSymbols())
}

func (cohortSample *CohortSample) gridIntervalAmong(symbols []string) int64 {
	deltas := make([]float64, 0, len(symbols)*8)

	for _, name := range symbols {
		symbolState := cohortSample.symbols[name]

		if symbolState == nil {
			continue
		}

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

/*
(…).sortedSymbols supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
func (cohortSample *CohortSample) sortedSymbols() []string {
	symbols := make([]string, 0, len(cohortSample.symbols))

	for symbol := range cohortSample.symbols {
		symbols = append(symbols, symbol)
	}

	sort.Strings(symbols)

	return symbols
}

/*
(…).currentTime supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
func (cohortSample *CohortSample) currentTime() int64 {
	return cohortSample.currentTimeAmong(cohortSample.sortedSymbols())
}

func (cohortSample *CohortSample) currentTimeAmong(symbols []string) int64 {
	current := int64(0)

	for _, name := range symbols {
		symbolState := cohortSample.symbols[name]

		if symbolState == nil || len(symbolState.times) == 0 {
			continue
		}

		last := symbolState.times[len(symbolState.times)-1]

		if last > current {
			current = last
		}
	}

	return current
}

/*
pairCorrelationCohort supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
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

/*
gridReturnsCohort supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
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

/*
pearsonCohort supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
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

/*
priceAtCohort forward-fills the last observed price at or before timestamp.
Synthetic grid cutoffs almost never equal exchange trade times.
*/
func priceAtCohort(symbolState *cohortSymbol, timestamp int64) (float64, bool) {
	if symbolState == nil || len(symbolState.times) == 0 {
		return 0, false
	}

	var price float64
	found := false

	for index, observed := range symbolState.times {
		if observed > timestamp {
			break
		}

		if index >= len(symbolState.prices) {
			continue
		}

		candidate := symbolState.prices[index]

		if candidate <= 0 {
			continue
		}

		price = candidate
		found = true
	}

	return price, found
}

/*
cohortFeatures supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
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

/*
appendRingInt64 supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
func appendRingInt64(values []int64, value int64, capacity int) []int64 {
	values = append(values, value)

	if len(values) <= capacity {
		return values
	}

	return values[len(values)-capacity:]
}

/*
logReturnsCohort supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
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

/*
cohortReturnInterval holds per-symbol sampling state that feeds equation Measure and Map outputs into resonance features.
*/
type cohortReturnInterval struct {
	start int64
	end   int64
	value float64
}

/*
returnIntervalsCohort supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
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

/*
hayashiYoshidaCohort supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
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

/*
medianAbsoluteIntervalsCohort supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
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
