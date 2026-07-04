package algorithm

import (
	"io"
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/causal"
)

const pearlSampleNodeCount = 4

/*
PearlSample turns Kraken trade and ticker frames into aligned node streams for Pearl.
Macro, liquidity, flow, and target nodes are derived from observed market fields.
*/
type PearlSample struct {
	artifact     *datura.Artifact
	pendingFrame bool
	output       []byte
	windows      map[string]*pearlWindow
	buffer       []byte
}

type pearlWindow struct {
	nodeRing      *causal.NodeRing
	lastMacro     float64
	lastLiquidity float64
	lastFlow      float64
	lastPrice     float64
}

/*
NewPearlSample returns a causal node encoder wired from a config artifact.
*/
func NewPearlSample(config *datura.Artifact) *PearlSample {
	return &PearlSample{
		artifact: config,
		windows:  map[string]*pearlWindow{},
		buffer:   make([]byte, 65536),
	}
}

func (pearlSample *PearlSample) Write(payload []byte) (int, error) {
	if len(payload) == 0 {
		pearlSample.pendingFrame = false
		pearlSample.output = nil

		return 0, nil
	}

	pearlSample.artifact.WithPayload(payload)
	pearlSample.pendingFrame = true
	pearlSample.output = nil

	return len(payload), nil
}

func (pearlSample *PearlSample) Read(payload []byte) (int, error) {
	if len(pearlSample.output) > 0 {
		return pearlSample.readOutput(payload)
	}

	if !pearlSample.pendingFrame {
		return 0, io.EOF
	}

	state := datura.Acquire("pearl-sample-state", datura.APPJSON)
	inbound := pearlSample.artifact.DecryptPayload()

	if len(inbound) == 0 {
		state.Release()
		pearlSample.pendingFrame = false

		return 0, io.EOF
	}

	if _, err := state.Unpack(inbound); err != nil {
		state.Release()
		pearlSample.pendingFrame = false

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"algorithm-pearl-sample: state write failed",
			err,
		))
	}

	defer state.Release()

	if datura.Peek[float64](state, "table", "rowCount") > 0 {
		pearlSample.output = state.Pack()

		return pearlSample.readOutput(payload)
	}

	row, window := pearlSample.ingestRow(state)

	if len(row) != pearlSampleNodeCount || window == nil {
		pearlSample.pendingFrame = false

		return 0, io.EOF
	}

	frame := datura.Acquire("pearl-node-frame", datura.APPJSON)
	frame.Poke(row, "batch")
	wire := frame.Pack()
	frame.Release()

	if len(wire) > 0 {
		_, _ = window.nodeRing.Write(wire)
		_, _ = window.nodeRing.Read(pearlSample.buffer)
	}

	window.nodeRing.CopyStreamsTo(state)

	pearlSample.output = state.Pack()

	return pearlSample.readOutput(payload)
}

func (pearlSample *PearlSample) readOutput(payload []byte) (int, error) {
	n := copy(payload, pearlSample.output)

	if n < len(pearlSample.output) {
		return n, io.ErrShortBuffer
	}

	pearlSample.output = nil
	pearlSample.pendingFrame = false

	return n, io.EOF
}

func (pearlSample *PearlSample) Close() error {
	return nil
}

func (pearlSample *PearlSample) ingestRow(state *datura.Artifact) ([]float64, *pearlWindow) {
	channel := datura.Peek[string](state, "channel")
	symbol := datura.Peek[string](state, "data", 0, "symbol")
	row := false

	if symbol == "" {
		symbol = datura.Peek[string](state, "symbol")
		row = symbol != ""
	}

	if symbol == "" {
		return nil, nil
	}

	if channel == "" && row {
		if datura.Peek[float64](state, "price") > 0 &&
			datura.Peek[float64](state, "qty") > 0 {
			channel = "trade"
		}

		if datura.Peek[float64](state, "last") > 0 {
			channel = "ticker"
		}

		if datura.Peek[float64](state, "bids", 0, "price") > 0 ||
			datura.Peek[float64](state, "asks", 0, "price") > 0 {
			channel = "book"
		}
	}

	window := pearlSample.window(symbol)

	switch channel {
	case "trade":
		return pearlSample.ingestTrade(state, window, row), window
	case "ticker":
		return pearlSample.ingestTicker(state, window, row), window
	case "book":
		return pearlSample.ingestBook(state, window, row), window
	}

	return nil, nil
}

func (pearlSample *PearlSample) ingestTrade(
	state *datura.Artifact,
	window *pearlWindow,
	row bool,
) []float64 {
	root := []any{"data", 0}
	if row {
		root = nil
	}

	price := peekFloat(state, root, "price")
	quantity := peekFloat(state, root, "qty")
	side := peekString(state, root, "side")

	if price <= 0 || quantity <= 0 {
		return nil
	}

	flow := quantity

	if side == "sell" {
		flow = -quantity
	}

	window.lastFlow = flow

	return []float64{
		window.lastMacro,
		window.lastLiquidity,
		flow,
		window.velocity(price),
	}
}

func (pearlSample *PearlSample) ingestTicker(
	state *datura.Artifact,
	window *pearlWindow,
	row bool,
) []float64 {
	root := []any{"data", 0}
	if row {
		root = nil
	}

	last := peekFloat(state, root, "last")
	changePct := peekFloat(state, root, "change_pct")

	if last <= 0 {
		return nil
	}

	liquidity := liquidityStress(
		peekFloat(state, root, "bid"),
		peekFloat(state, root, "ask"),
		peekFloat(state, root, "bid_qty"),
		peekFloat(state, root, "ask_qty"),
	)

	if liquidity > 0 {
		window.lastLiquidity = liquidity
	}

	window.lastMacro = changePct / 100

	return []float64{
		window.lastMacro,
		window.lastLiquidity,
		window.lastFlow,
		window.velocity(last),
	}
}

func (pearlSample *PearlSample) ingestBook(
	state *datura.Artifact,
	window *pearlWindow,
	row bool,
) []float64 {
	root := []any{"data", 0}
	if row {
		root = nil
	}

	bid, bidQty := bestBid(state, root)
	ask, askQty := bestAsk(state, root)
	liquidity := liquidityStress(bid, ask, bidQty, askQty)

	if liquidity <= 0 {
		return nil
	}

	window.lastLiquidity = liquidity

	return []float64{
		window.lastMacro,
		liquidity,
		window.lastFlow,
		window.velocity((bid + ask) / 2),
	}
}

func (window *pearlWindow) velocity(price float64) float64 {
	velocity := 0.0

	if window.lastPrice > 0 && price > 0 {
		velocity = math.Log(price / window.lastPrice)
	}

	if price > 0 {
		window.lastPrice = price
	}

	return velocity
}

func (pearlSample *PearlSample) window(symbol string) *pearlWindow {
	if window, ok := pearlSample.windows[symbol]; ok {
		return window
	}

	capacity := datura.Peek[float64](pearlSample.artifact, "history")

	if capacity <= 0 {
		capacity = datura.Peek[float64](pearlSample.artifact, "minHistory")
	}

	window := &pearlWindow{
		nodeRing: causal.NewNodeRing(
			datura.Acquire("pearl-node-ring", datura.APPJSON).WithAttributes(datura.Map[any]{
				"nodeCount": float64(pearlSampleNodeCount),
				"capacity":  capacity,
			}).WithPayload([]byte("{}")),
		),
	}

	pearlSample.windows[symbol] = window

	return window
}

func liquidityStress(bid, ask, bidQty, askQty float64) float64 {
	if bid <= 0 || ask <= 0 || ask <= bid {
		return 0
	}

	depth := bidQty + askQty

	if depth <= 0 {
		return 0
	}

	return (ask - bid) / depth
}

func bestBid(state *datura.Artifact, root []any) (price, quantity float64) {
	for index := 0; ; index++ {
		levelRoot := append(append([]any{}, root...), "bids", index)
		levelPrice := datura.Peek[float64](state, append(levelRoot, "price")...)

		if levelPrice <= 0 {
			return price, quantity
		}

		if levelPrice > price {
			price = levelPrice
			quantity = datura.Peek[float64](state, append(levelRoot, "qty")...)
		}
	}
}

func bestAsk(state *datura.Artifact, root []any) (price, quantity float64) {
	for index := 0; ; index++ {
		levelRoot := append(append([]any{}, root...), "asks", index)
		levelPrice := datura.Peek[float64](state, append(levelRoot, "price")...)

		if levelPrice <= 0 {
			return price, quantity
		}

		if price == 0 || levelPrice < price {
			price = levelPrice
			quantity = datura.Peek[float64](state, append(levelRoot, "qty")...)
		}
	}
}

func peekFloat(state *datura.Artifact, root []any, key string) float64 {
	path := append(append([]any{}, root...), key)

	value := datura.Peek[float64](state, path...)

	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}

	return value
}

func peekString(state *datura.Artifact, root []any, key string) string {
	path := append(append([]any{}, root...), key)

	return datura.Peek[string](state, path...)
}
