package runtime

import (
	"github.com/bytedance/sonic"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/algorithm"
	"github.com/theapemachine/nomagique/algorithm/excitation"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/market"
)

type Envelope struct {
	*market.Trade `json:"trade"`
	Correlation   map[string]*equation.CohortOutput      `json:"correlation"`
	CVD           map[string]*equation.FlowOutput        `json:"cvd"`
	DepthFlow     map[string]*equation.BookflowOutput    `json:"depthflow"`
	Hawkes        map[string]*excitation.Outcome         `json:"hawkes"`
	LeadLag       map[string]*algorithm.LagOutcome       `json:"leadlag"`
	Liquidity     map[string]*equation.DepthOutput       `json:"liquidity"`
	Morphology    map[string]*equation.MorphologyOutput  `json:"morphology"`
	Pumpdump      map[string]*equation.IgnitionOutput    `json:"pumpdump"`
	Sentiment     map[string]*equation.ConvictionOutput  `json:"sentiment"`
	Toxicity      map[string]*equation.BookQualityOutput `json:"toxicity"`
	Resonance     map[string]map[string]float64          `json:"resonance"`
	Regions       map[string][]*Region                   `json:"regions"`
	Paths         map[string]*Tokens                     `json:"paths"`
}

func NewEnvelope(trade *market.Trade) *Envelope {
	return &Envelope{
		Trade:       trade,
		Correlation: make(map[string]*equation.CohortOutput),
		CVD:         make(map[string]*equation.FlowOutput),
		DepthFlow:   make(map[string]*equation.BookflowOutput),
		Hawkes:      make(map[string]*excitation.Outcome),
		LeadLag:     make(map[string]*algorithm.LagOutcome),
		Liquidity:   make(map[string]*equation.DepthOutput),
		Morphology:  make(map[string]*equation.MorphologyOutput),
		Pumpdump:    make(map[string]*equation.IgnitionOutput),
		Sentiment:   make(map[string]*equation.ConvictionOutput),
		Toxicity:    make(map[string]*equation.BookQualityOutput),
		Resonance:   make(map[string]map[string]float64),
		Regions:     make(map[string][]*Region),
		Paths:       make(map[string]*Tokens),
	}
}

/*
Metrics gathers every symbol's mapped signal readings into the
symbol → source → label → value shape grid.Observe expects.
*/
func (envelope *Envelope) Metrics() map[string]map[string]map[string]float64 {
	out := make(map[string]map[string]map[string]float64)

	if envelope == nil {
		return out
	}

	put := func(symbol, source string, labels map[string]float64) {
		if symbol == "" || len(labels) == 0 {
			return
		}

		sources := out[symbol]
		if sources == nil {
			sources = make(map[string]map[string]float64)
			out[symbol] = sources
		}

		sources[source] = labels
	}

	for symbol, reading := range envelope.Correlation {
		put(symbol, "correlation", reading.Map())
	}
	for symbol, reading := range envelope.CVD {
		put(symbol, "cvd", reading.Map())
	}
	for symbol, reading := range envelope.DepthFlow {
		put(symbol, "depthflow", reading.Map())
	}
	for symbol, reading := range envelope.Hawkes {
		put(symbol, "hawkes", reading.Map())
	}
	for symbol, reading := range envelope.LeadLag {
		put(symbol, "leadlag", reading.Map())
	}
	for symbol, reading := range envelope.Liquidity {
		put(symbol, "liquidity", reading.Map())
	}
	for symbol, reading := range envelope.Morphology {
		put(symbol, "morphology", reading.Map())
	}
	for symbol, reading := range envelope.Pumpdump {
		put(symbol, "pumpdump", reading.Map())
	}
	for symbol, reading := range envelope.Sentiment {
		put(symbol, "sentiment", reading.Map())
	}
	for symbol, reading := range envelope.Toxicity {
		put(symbol, "toxicity", reading.Map())
	}
	for symbol, reading := range envelope.Resonance {
		put(symbol, "resonance", reading)
	}

	return out
}

/*
Publish returns JSON {"<ctx>": facet} for the dashboard store keyed by ctx.
*/
func (envelope *Envelope) Publish(ctx string) []byte {
	var payload any

	switch ctx {
	case "regions":
		payload = envelope.Regions
	case "paths":
		payload = envelope.Paths
	case "resonance":
		payload = envelope.Resonance
	case "trade":
		payload = envelope.Trade
	case "correlation":
		payload = envelope.Correlation
	case "cvd":
		payload = envelope.CVD
	case "depthflow":
		payload = envelope.DepthFlow
	case "hawkes":
		payload = envelope.Hawkes
	case "leadlag":
		payload = envelope.LeadLag
	case "liquidity":
		payload = envelope.Liquidity
	case "morphology":
		payload = envelope.Morphology
	case "pumpdump":
		payload = envelope.Pumpdump
	case "sentiment":
		payload = envelope.Sentiment
	case "toxicity":
		payload = envelope.Toxicity
	default:
		payload = envelope
	}

	if payload == nil {
		return nil
	}

	buf, err := sonic.Marshal(map[string]any{ctx: payload})

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.IO,
			"runtime: envelope publish failed",
			err,
		))

		return nil
	}

	return buf
}
