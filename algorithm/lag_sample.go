package algorithm

import (
	"math"
	"sync"
	"time"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/correlation"
)

const (
	lagSampleHistoryCap   = 128
	lagSampleMinObs       = 8
	lagSampleBarInterval  = time.Second
	lagSampleMaxInterval  = 30 * time.Second
	lagSampleBaselineMin  = 8
)

/*
LagSample turns symbol price streams into the LagInput classifier expects.
It owns path retention, move baselines, and peer cross-lag / contemporaneous
correlation so consumers only call Measure.
*/
type LagSample struct {
	config  LagSampleConfig
	symbols *sync.Map
}

/*
LagSampleConfig controls structural history retention and bar spacing.
*/
type LagSampleConfig struct {
	HistoryCap   int
	MinObs       int
	BarInterval  time.Duration
	MaxInterval  time.Duration
	BaselineMin  int
}

/*
LagSampleInput is one observed ticker price.
*/
type LagSampleInput struct {
	Symbol string
	Price  float64
	At     time.Time
}

type lagSymbol struct {
	mu       sync.Mutex
	samples  []correlation.Sample
	last     float64
	baseline *MoveBaseline
}

/*
NewLagSample returns a typed lead-lag sampler.
*/
func NewLagSample(config ...LagSampleConfig) *LagSample {
	return &LagSample{
		config:  lagSampleConfig(config),
		symbols: &sync.Map{},
	}
}

func lagSampleConfig(config []LagSampleConfig) LagSampleConfig {
	resolved := LagSampleConfig{
		HistoryCap:  lagSampleHistoryCap,
		MinObs:      lagSampleMinObs,
		BarInterval: lagSampleBarInterval,
		MaxInterval: lagSampleMaxInterval,
		BaselineMin: lagSampleBaselineMin,
	}

	if len(config) == 0 {
		return resolved
	}

	if config[0].HistoryCap > 0 {
		resolved.HistoryCap = config[0].HistoryCap
	}

	if config[0].MinObs > 0 {
		resolved.MinObs = config[0].MinObs
	}

	if config[0].BarInterval > 0 {
		resolved.BarInterval = config[0].BarInterval
	}

	if config[0].MaxInterval > 0 {
		resolved.MaxInterval = config[0].MaxInterval
	}

	if config[0].BaselineMin > 0 {
		resolved.BaselineMin = config[0].BaselineMin
	}

	return resolved
}

/*
Measure observes one price print and returns lag input once the symbol's path
and move baseline can support classification.
*/
func (sample *LagSample) Measure(input LagSampleInput) (LagInput, bool, error) {
	if input.Symbol == "" || input.Price <= 0 || input.At.IsZero() {
		return LagInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag-sample: trade requires symbol, positive price, and timestamp",
			nil,
		))
	}

	if math.IsNaN(input.Price) || math.IsInf(input.Price, 0) {
		return LagInput{}, false, errnie.Error(errnie.Err(
			errnie.Validation,
			"lag-sample: price is non-finite",
			nil,
		))
	}

	symbol := sample.symbol(input.Symbol)
	symbol.mu.Lock()
	defer symbol.mu.Unlock()

	move := 0.0
	if symbol.last > 0 {
		move = math.Abs(math.Log(input.Price / symbol.last))
	}

	symbol.last = input.Price
	symbol.samples = append(symbol.samples, correlation.Sample{
		At:    input.At,
		Value: input.Price,
	})

	if len(symbol.samples) > sample.config.HistoryCap {
		symbol.samples = symbol.samples[len(symbol.samples)-sample.config.HistoryCap:]
	}

	own := append([]correlation.Sample(nil), symbol.samples...)

	gate, err := symbol.baseline.Measure(move)
	if err != nil {
		return LagInput{}, false, err
	}

	if gate.Ready <= 0 && len(own) < sample.config.MinObs {
		return LagInput{}, false, nil
	}

	peer, peerOK := sample.peer(input.Symbol)
	lagBars, lagCorr, lagOK := 0, 0.0, false
	contempCorr, contempOK := 0.0, false

	if peerOK {
		lagBars, lagCorr, lagOK = CrossLagScore(peer, own, sample.config.BarInterval)
		contempCorr, contempOK = HayashiPairCorrelation(peer, own, sample.config.MaxInterval)
	}

	return LagInput{
		IsAnchor:    !peerOK && gate.Ready > 0,
		Price:       input.Price,
		MoveReady:   gate.Ready > 0,
		MoveMoved:   gate.Moved > 0,
		StallMargin: gate.StallMargin,
		LagOK:       lagOK,
		LagBars:     lagBars,
		LagCorr:     lagCorr,
		ContempOK:   contempOK,
		ContempCorr: contempCorr,
		SampleCount: len(own),
	}, true, nil
}

func (sample *LagSample) symbol(name string) *lagSymbol {
	if existing, ok := sample.symbols.Load(name); ok {
		return existing.(*lagSymbol)
	}

	created := &lagSymbol{
		baseline: NewMoveBaseline(MoveBaselineConfig{
			MinObs:  sample.config.BaselineMin,
			PathCap: sample.config.HistoryCap,
		}),
	}
	actual, _ := sample.symbols.LoadOrStore(name, created)

	return actual.(*lagSymbol)
}

func (sample *LagSample) peer(symbol string) ([]correlation.Sample, bool) {
	var peer []correlation.Sample

	sample.symbols.Range(func(key, untyped any) bool {
		if key.(string) == symbol {
			return true
		}

		other := untyped.(*lagSymbol)
		other.mu.Lock()
		defer other.mu.Unlock()

		if len(other.samples) < sample.config.MinObs {
			return true
		}

		peer = append([]correlation.Sample(nil), other.samples...)
		return false
	})

	return peer, len(peer) >= sample.config.MinObs
}
