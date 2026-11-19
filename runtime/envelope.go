package runtime

import (
	"sync"

	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/algorithm"
	"github.com/theapemachine/nomagique/algorithm/excitation"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/market"
)

/*
Decision is one symbol's model/desk outcome for a tick: action taken or why not.
*/
type Decision struct {
	Symbol     string  `json:"symbol"`
	Action     string  `json:"action"`
	Reason     string  `json:"reason,omitempty"`
	Gate       string  `json:"gate,omitempty"`
	Path       string  `json:"path,omitempty"`
	Tier       string  `json:"tier,omitempty"`
	Resistance float64 `json:"resistance,omitempty"`
}

/*
Envelope is the one disruptor payload per tick: symm nodes read and enrich the
same struct in ring order, embed the market Trade anchor, and fan signal output
into store.Grid via Metrics and dashboard facets via Publish.
*/
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
	Decisions     map[string]*Decision                   `json:"decisions"`
	N             uint64                                 `json:"n,omitempty"`
	// metricsCache is built once per tick; Resonance is overlaid on later reads
	// so training sees solver output without rebuilding every signal Map().
	metricsCache map[string]map[string]map[string]float64 `json:"-"`
	// decideMu guards Decisions: Model.Step fans Decide across symbols.
	decideMu sync.Mutex `json:"-"`
}

/*
NewEnvelope pre-allocates per-source maps so parallel pipeline nodes can write
without nil checks on the hot tick path.
*/
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
		Decisions:   make(map[string]*Decision),
	}
}

/*
Decide records the outcome for one symbol (and tier, when set) on this tick.
Safe under Model.Step's per-symbol goroutine fan-out.
*/
func (envelope *Envelope) Decide(decision *Decision) {
	if envelope == nil || decision == nil || decision.Symbol == "" {
		return
	}

	envelope.decideMu.Lock()
	defer envelope.decideMu.Unlock()

	if envelope.Decisions == nil {
		envelope.Decisions = make(map[string]*Decision)
	}

	key := decision.Symbol

	if decision.Tier != "" {
		key = decision.Symbol + "/" + decision.Tier
	}

	envelope.Decisions[key] = decision
}

/*
Metrics gathers every symbol's mapped signal readings into the
symbol → source → label → value shape grid.Observe expects.
Built once per envelope; later calls only overlay Resonance.
Observe may take ownership of the inner label maps.
*/
func (envelope *Envelope) Metrics() map[string]map[string]map[string]float64 {
	if envelope == nil {
		return make(map[string]map[string]map[string]float64)
	}

	if envelope.metricsCache == nil {
		envelope.metricsCache = envelope.buildMetrics()
	} else {
		envelope.overlayResonance(envelope.metricsCache)
	}

	return envelope.metricsCache
}

/*
putMetric inserts one source's label map into the symbol→source→label cube
grid.Observe and tick persistence both consume.
*/
func putMetric(
	out map[string]map[string]map[string]float64,
	symbol, source string,
	labels map[string]float64,
) {
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

/*
buildMetrics runs once per tick to fan every signal Map() into metricsCache
before resonance overlay and store observation.
*/
func (envelope *Envelope) buildMetrics() map[string]map[string]map[string]float64 {
	out := make(map[string]map[string]map[string]float64)

	for symbol, reading := range envelope.Correlation {
		putMetric(out, symbol, "correlation", reading.Map())
	}
	for symbol, reading := range envelope.CVD {
		putMetric(out, symbol, "cvd", reading.Map())
	}
	for symbol, reading := range envelope.DepthFlow {
		putMetric(out, symbol, "depthflow", reading.Map())
	}
	for symbol, reading := range envelope.Hawkes {
		putMetric(out, symbol, "hawkes", reading.Map())
	}
	for symbol, reading := range envelope.LeadLag {
		putMetric(out, symbol, "leadlag", reading.Map())
	}
	for symbol, reading := range envelope.Liquidity {
		putMetric(out, symbol, "liquidity", reading.Map())
	}
	for symbol, reading := range envelope.Morphology {
		putMetric(out, symbol, "morphology", reading.Map())
	}
	for symbol, reading := range envelope.Pumpdump {
		putMetric(out, symbol, "pumpdump", reading.Map())
	}
	for symbol, reading := range envelope.Sentiment {
		putMetric(out, symbol, "sentiment", reading.Map())
	}
	for symbol, reading := range envelope.Toxicity {
		putMetric(out, symbol, "toxicity", reading.Map())
	}

	envelope.overlayResonance(out)

	return out
}

/*
overlayResonance merges solver output after the first Metrics read so training
sees resonance without rebuilding the full signal cube every call.
*/
func (envelope *Envelope) overlayResonance(
	out map[string]map[string]map[string]float64,
) {
	for symbol, reading := range envelope.Resonance {
		putMetric(out, symbol, "resonance", reading)
	}
}

/*
Publish returns JSON {"<ctx>": facet} for the dashboard store keyed by ctx.
*/
func (envelope *Envelope) Publish(ctx string, focusSymbol string) []byte {
	var payload any

	switch ctx {
	case "regions":
		payload = envelope.Regions
	case "paths":
		payload = envelope.Paths
	case "resonance":
		payload = envelope.Resonance
	case "decisions":
		payload = envelope.Decisions
	case "trade":
		payload = envelope.Trade
	case "correlation":
		payload = envelope.Correlation[focusSymbol]
	case "cvd":
		payload = envelope.CVD[focusSymbol]
	case "depthflow":
		payload = envelope.DepthFlow[focusSymbol]
	case "hawkes":
		payload = envelope.Hawkes[focusSymbol]
	case "leadlag":
		payload = envelope.LeadLag[focusSymbol]
	case "liquidity":
		payload = envelope.Liquidity[focusSymbol]
	case "morphology":
		payload = envelope.Morphology[focusSymbol]
	case "pumpdump":
		payload = envelope.Pumpdump[focusSymbol]
	case "sentiment":
		payload = envelope.Sentiment[focusSymbol]
	case "toxicity":
		payload = envelope.Toxicity[focusSymbol]
	default:
		payload = envelope
	}

	if payload == nil {
		return nil
	}

	buf, err := Sonic.Marshal(map[string]any{ctx: payload})

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
