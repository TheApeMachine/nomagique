package algorithm

import (
	"math"
	"sync"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

const (
	depthMinPeers      = 2
	depthHistoryCap    = 64
	depthRelativeFloor = 1e-12
)

/*
DepthSample turns executable touch depth into Depth inputs against a live peer
cohort.
*/
type DepthSample struct {
	symbols *sync.Map
}

/*
DepthSampleInput is one two-sided touch observation.
*/
type DepthSampleInput struct {
	Symbol string
	Bid    float64
	Ask    float64
	BidQty float64
	AskQty float64
	At     time.Time
}

type depthSymbol struct {
	mu      sync.Mutex
	at      time.Time
	depth   float64
	history []float64
	cadence time.Duration
}

/*
NewDepthSample returns a cross-section liquidity depth sampler.
*/
func NewDepthSample() *DepthSample {
	return &DepthSample{symbols: &sync.Map{}}
}

/*
Measure observes one touch and emits depth input once peer depth exists.
*/
func (sample *DepthSample) Measure(
	input DepthSampleInput,
) (equation.DepthInput, bool, error) {
	if input.Symbol == "" || input.At.IsZero() {
		return equation.DepthInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"depth-sample: symbol and timestamp required",
			nil,
		))
	}

	depth := executableTouchDepth(input)
	if depth <= 0 {
		return equation.DepthInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"depth-sample: executable touch depth unavailable",
			nil,
		))
	}

	symbol := sample.symbol(input.Symbol)
	symbol.mu.Lock()

	if !symbol.at.IsZero() && !input.At.After(symbol.at) {
		symbol.mu.Unlock()
		return equation.DepthInput{}, false, nil
	}

	if !symbol.at.IsZero() {
		symbol.cadence = input.At.Sub(symbol.at)
	}

	symbol.at = input.At
	symbol.depth = depth
	symbol.history = append(symbol.history, depth)
	if len(symbol.history) > depthHistoryCap {
		symbol.history = symbol.history[len(symbol.history)-depthHistoryCap:]
	}

	history := append([]float64(nil), symbol.history...)
	symbol.mu.Unlock()

	peers, ok := sample.peerDepths(input.Symbol)
	if !ok {
		return equation.DepthInput{}, false, nil
	}

	relative := 0.0
	baselineReady := false
	if medianOwn, medianOK := statistic.MedianOf(history); medianOK && medianOwn > depthRelativeFloor {
		relative = depth / medianOwn
		baselineReady = len(history) >= depthMinPeers
	}

	return equation.DepthInput{
		QuoteVolume:    depth,
		Peers:          peers,
		RelativeVolume: relative,
		BaselineReady:  baselineReady,
	}, true, nil
}

func executableTouchDepth(input DepthSampleInput) float64 {
	if input.BidQty <= 0 || input.AskQty <= 0 ||
		input.Bid <= 0 || input.Ask <= input.Bid ||
		math.IsNaN(input.Bid) || math.IsNaN(input.Ask) ||
		math.IsInf(input.Bid, 0) || math.IsInf(input.Ask, 0) {
		return 0
	}

	return math.Min(input.BidQty, input.AskQty) * (input.Bid + input.Ask) / 2
}

func (sample *DepthSample) peerDepths(focal string) ([]float64, bool) {
	type peer struct {
		depth   float64
		at      time.Time
		cadence time.Duration
	}

	peers := make([]peer, 0)
	cadences := make([]float64, 0)
	latest := time.Time{}

	sample.symbols.Range(func(key, untyped any) bool {
		if key.(string) == focal {
			return true
		}

		state := untyped.(*depthSymbol)
		state.mu.Lock()
		defer state.mu.Unlock()

		if state.depth <= 0 || state.at.IsZero() {
			return true
		}

		if state.at.After(latest) {
			latest = state.at
		}
		if state.cadence > 0 {
			cadences = append(cadences, float64(state.cadence))
		}

		peers = append(peers, peer{depth: state.depth, at: state.at, cadence: state.cadence})
		return true
	})

	if len(peers) < depthMinPeers {
		return nil, false
	}

	freshness := time.Duration(0)
	if medianCadence, ok := statistic.MedianOf(cadences); ok && medianCadence > 0 {
		freshness = time.Duration(medianCadence)
	}

	depths := make([]float64, 0, len(peers))
	for _, peer := range peers {
		if freshness > 0 && latest.Sub(peer.at) > freshness {
			continue
		}
		depths = append(depths, peer.depth)
	}

	return depths, len(depths) >= depthMinPeers
}

func (sample *DepthSample) symbol(name string) *depthSymbol {
	if existing, ok := sample.symbols.Load(name); ok {
		return existing.(*depthSymbol)
	}

	created := &depthSymbol{}
	actual, _ := sample.symbols.LoadOrStore(name, created)
	return actual.(*depthSymbol)
}
