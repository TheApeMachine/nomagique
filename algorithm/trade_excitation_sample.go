package algorithm

import (
	"fmt"
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

const excitationSampleHistoryCap = 128
const excitationSampleSideCap = 64

/*
TradeExcitationSample accumulates trade arrival times into the batch Excitation expects.
Horizon and fit cooldown are derived from the observed arrival span.
*/
type TradeExcitationSample struct {
	artifact *datura.Artifact
	windows  map[string]*tradeExcitationWindow
}

type tradeExcitationWindow struct {
	buySeconds  []float64
	sellSeconds []float64
}

/*
NewTradeExcitationSample returns a trade arrival encoder for Hawkes excitation.
*/
func NewTradeExcitationSample(artifact *datura.Artifact) *TradeExcitationSample {
	return &TradeExcitationSample{
		artifact: artifact,
		windows:  map[string]*tradeExcitationWindow{},
	}
}

func (tradeExcitationSample *TradeExcitationSample) Write(payload []byte) (int, error) {
	tradeExcitationSample.artifact.WithPayload(payload)
	return len(payload), nil
}

func (tradeExcitationSample *TradeExcitationSample) Read(payload []byte) (int, error) {
	state := datura.Acquire("trade-excitation-sample-state", datura.APPJSON)

	if _, err := state.Write(tradeExcitationSample.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	channel := datura.Peek[string](state, "channel")

	if channel != "trade" {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			fmt.Sprintf("trade-excitation-sample: expected trade channel, got %q", channel),
			nil,
		))
	}

	symbol := datura.Peek[string](state, "data", 0, "symbol")
	side := datura.Peek[string](state, "data", 0, "side")

	if symbol == "" || (side != "buy" && side != "sell") {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"trade-excitation-sample: trade frame requires symbol and buy/sell side",
			nil,
		))
	}

	arrival := tradeExcitationSample.arrivalSecond(state)
	window := tradeExcitationSample.window(symbol)

	if side == "buy" {
		window.buySeconds = append(window.buySeconds, arrival)
	}

	if side == "sell" {
		window.sellSeconds = append(window.sellSeconds, arrival)
	}

	if len(window.buySeconds) > excitationSampleSideCap {
		window.buySeconds = window.buySeconds[len(window.buySeconds)-excitationSampleSideCap:]
	}

	if len(window.sellSeconds) > excitationSampleSideCap {
		window.sellSeconds = window.sellSeconds[len(window.sellSeconds)-excitationSampleSideCap:]
	}

	tradeExcitationSample.trimWindow(window)

	features := tradeExcitationSample.features(window)

	if len(features) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"trade-excitation-sample: insufficient trade history for excitation features",
			nil,
		))
	}

	state.WithScope(symbol)
	state.Merge("features", features)
	state.Merge("root", "features")
	state.Merge("inputs", ExcitationSampleInputKeys)
	state.Poke(float64(len(window.buySeconds)), "config", "xCount")
	state.Poke(float64(len(window.sellSeconds)), "config", "yCount")

	return state.Read(payload)
}

func (tradeExcitationSample *TradeExcitationSample) Close() error {
	return nil
}

func (tradeExcitationSample *TradeExcitationSample) window(symbol string) *tradeExcitationWindow {
	existing, ok := tradeExcitationSample.windows[symbol]

	if ok {
		return existing
	}

	window := &tradeExcitationWindow{}
	tradeExcitationSample.windows[symbol] = window

	return window
}

func (tradeExcitationSample *TradeExcitationSample) arrivalSecond(state *datura.Artifact) float64 {
	timestamp := state.Timestamp()

	if timestamp <= 0 {
		return 0
	}

	return float64(timestamp) / float64(time.Second)
}

func (tradeExcitationSample *TradeExcitationSample) trimWindow(window *tradeExcitationWindow) {
	total := len(window.buySeconds) + len(window.sellSeconds)

	if total <= excitationSampleHistoryCap {
		return
	}

	trim := total - excitationSampleHistoryCap

	for trim > 0 && len(window.buySeconds) > 0 {
		window.buySeconds = window.buySeconds[1:]
		trim--
	}

	for trim > 0 && len(window.sellSeconds) > 0 {
		window.sellSeconds = window.sellSeconds[1:]
		trim--
	}
}

func (tradeExcitationSample *TradeExcitationSample) features(window *tradeExcitationWindow) []float64 {
	eventCount := len(window.buySeconds) + len(window.sellSeconds)

	if eventCount < excitationSampleMinEvents(eventCount) {
		return nil
	}

	allSeconds := append([]float64(nil), window.buySeconds...)
	allSeconds = append(allSeconds, window.sellSeconds...)

	if len(allSeconds) == 0 {
		return nil
	}

	first := allSeconds[0]
	last := allSeconds[0]

	for _, second := range allSeconds[1:] {
		if second < first {
			first = second
		}

		if second > last {
			last = second
		}
	}

	span := time.Duration((last - first) * float64(time.Second))

	if span <= 0 {
		span = time.Second
	}

	horizon := last
	cooldown := DeriveFitCooldown(span).Seconds()

	features := []float64{
		horizon,
		cooldown,
		float64(len(window.buySeconds)),
		float64(len(window.sellSeconds)),
	}
	features = append(features, window.buySeconds...)
	features = append(features, window.sellSeconds...)

	return features
}

func excitationSampleMinEvents(eventCount int) int {
	required := int(math.Ceil(math.Sqrt(float64(eventCount))))

	if required < 8 {
		return 8
	}

	return required
}
