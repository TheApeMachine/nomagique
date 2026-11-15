package algorithm

import (
	"math"
	"sync"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/statistic"
)

/*
defaultSurgeGate bounds sampling or gates so equation feature frames stay well-posed for symm envelope wiring.
*/
/*
convictionMinPeers bounds sampling or gates so equation feature frames stay well-posed for symm envelope wiring.
*/
const (
	convictionMinPeers  = 2
	defaultSurgeGate    = 0.5
	convictionFreshness = 30 * time.Second
)

/*
ConvictionSample turns symbol price streams into Conviction inputs.
*/
type ConvictionSample struct {
	symbols *sync.Map
}

/*
ConvictionSampleInput is one observed ticker price.
*/
type ConvictionSampleInput struct {
	Symbol string
	Price  float64
	At     time.Time
}

/*
convictionSymbol holds per-symbol sampling state that feeds equation Measure and Map outputs into resonance features.
*/
type convictionSymbol struct {
	mu      sync.Mutex
	last    float64
	at      time.Time
	change  float64
	ready   bool
	cadence time.Duration
}

/*
NewConvictionSample returns a board-breadth conviction sampler.
*/
func NewConvictionSample() *ConvictionSample {
	return &ConvictionSample{symbols: &sync.Map{}}
}

/*
Measure observes one price and emits conviction input once a peer cohort exists.
*/
func (sample *ConvictionSample) Measure(
	input ConvictionSampleInput,
) (equation.ConvictionInput, bool, error) {
	if input.Symbol == "" || input.Price <= 0 || input.At.IsZero() {
		return equation.ConvictionInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"conviction-sample: symbol, positive price, and timestamp required",
			nil,
		))
	}

	if math.IsNaN(input.Price) || math.IsInf(input.Price, 0) {
		return equation.ConvictionInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"conviction-sample: price is non-finite",
			nil,
		))
	}

	symbol := sample.symbol(input.Symbol)
	symbol.mu.Lock()

	if !symbol.at.IsZero() && !input.At.After(symbol.at) {
		symbol.mu.Unlock()
		return equation.ConvictionInput{}, false, nil
	}

	if symbol.last > 0 {
		symbol.change = math.Log(input.Price / symbol.last)
		if !symbol.at.IsZero() {
			symbol.cadence = input.At.Sub(symbol.at)
		}
		symbol.ready = true
	}

	symbol.last = input.Price
	symbol.at = input.At
	change := symbol.change
	ready := symbol.ready
	symbol.mu.Unlock()

	if !ready {
		return equation.ConvictionInput{}, false, nil
	}

	stats, ok := sample.statistics()
	if !ok {
		return equation.ConvictionInput{}, false, nil
	}

	return equation.ConvictionInput{
		Breadth:         stats.breadth,
		Change:          change,
		SurgeThreshold:  stats.surgeThreshold,
		ChangeThreshold: stats.changeThreshold,
		Leader:          stats.leader == input.Symbol && stats.leaderMagnitude > 0,
	}, true, nil
}

/*
convictionStats holds per-symbol sampling state that feeds equation Measure and Map outputs into resonance features.
*/
type convictionStats struct {
	leader          string
	leaderMagnitude float64
	breadth         float64
	surgeThreshold  float64
	changeThreshold float64
}

/*
(…).statistics supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
func (sample *ConvictionSample) statistics() (convictionStats, bool) {
	type peer struct {
		symbol  string
		change  float64
		at      time.Time
		cadence time.Duration
	}

	peers := make([]peer, 0)
	latest := time.Time{}

	sample.symbols.Range(func(key, untyped any) bool {
		state := untyped.(*convictionSymbol)
		state.mu.Lock()
		defer state.mu.Unlock()

		if !state.ready {
			return true
		}

		if state.at.After(latest) {
			latest = state.at
		}

		peers = append(peers, peer{
			symbol:  key.(string),
			change:  state.change,
			at:      state.at,
			cadence: state.cadence,
		})

		return true
	})

	if len(peers) < convictionMinPeers {
		return convictionStats{}, false
	}

	alive := make([]peer, 0, len(peers))
	for _, peer := range peers {
		limit := convictionFreshness

		if peer.cadence > limit {
			limit = peer.cadence
		}

		if latest.Sub(peer.at) > limit {
			continue
		}

		alive = append(alive, peer)
	}

	if len(alive) < convictionMinPeers {
		return convictionStats{}, false
	}

	stats := convictionStats{surgeThreshold: defaultSurgeGate}
	advances, declines := 0, 0
	magnitudes := make([]float64, 0, len(alive))

	for _, peer := range alive {
		magnitude := math.Abs(peer.change)
		magnitudes = append(magnitudes, magnitude)

		if peer.change > 0 {
			advances++
		}
		if peer.change < 0 {
			declines++
		}
		if magnitude > stats.leaderMagnitude {
			stats.leader = peer.symbol
			stats.leaderMagnitude = magnitude
		}
	}

	stats.breadth = float64(advances-declines) / float64(len(alive))

	if medianMagnitude, ok := statistic.MedianOf(magnitudes); ok && medianMagnitude > 0 {
		stats.changeThreshold = medianMagnitude
	}

	return stats, true
}

/*
(…).symbol supports algorithm sampling that supplies equation inputs symm maps into runtime.Envelope.
*/
func (sample *ConvictionSample) symbol(name string) *convictionSymbol {
	if existing, ok := sample.symbols.Load(name); ok {
		return existing.(*convictionSymbol)
	}

	created := &convictionSymbol{}
	actual, _ := sample.symbols.LoadOrStore(name, created)
	return actual.(*convictionSymbol)
}
