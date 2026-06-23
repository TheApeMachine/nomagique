package algorithm

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/causal"
)

const pearlSampleNodeCount = 4

/*
PearlSample turns Kraken trade and ticker frames into aligned node streams for Pearl.
Macro, liquidity, flow, and target nodes are derived from observed market fields.
*/
type PearlSample struct {
	artifact      *datura.Artifact
	nodeRing      *causal.NodeRing
	lastMacro     float64
	lastLiquidity float64
	lastFlow      float64
}

/*
NewPearlSample returns a causal node encoder wired from a config artifact.
*/
func NewPearlSample(config *datura.Artifact) *PearlSample {
	nodeRing := causal.NewNodeRing(
		datura.Acquire("pearl-node-ring", datura.APPJSON).
			Poke(float64(pearlSampleNodeCount), "nodeCount").
			Poke(datura.Peek[float64](config, "minHistory"), "capacity"),
	)

	return &PearlSample{
		artifact: config,
		nodeRing: nodeRing,
	}
}

func (pearlSample *PearlSample) Write(payload []byte) (int, error) {
	pearlSample.artifact.WithPayload(payload)
	return len(payload), nil
}

func (pearlSample *PearlSample) Read(payload []byte) (int, error) {
	state := datura.Acquire("pearl-sample-state", datura.APPJSON)

	if _, err := state.Write(pearlSample.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, err
	}

	state.Inspect("algorithm", "pearl-sample", "Read()", "p")

	defer state.Release()

	if datura.Peek[float64](state, "table", "rowCount") > 0 {
		return state.Read(payload)
	}

	row := pearlSample.ingestRow(state)

	if len(row) == pearlSampleNodeCount {
		frame := datura.Acquire("pearl-node-frame", datura.APPJSON)
		frame.Poke(row, "batch")
		wire := frame.Pack()
		frame.Release()

		if len(wire) > 0 {
			_, _ = pearlSample.nodeRing.Write(wire)

			ringBuffer := make([]byte, 65536)
			_, _ = pearlSample.nodeRing.Read(ringBuffer)
		}
	}

	pearlSample.nodeRing.CopyStreamsTo(state)

	return state.Read(payload)
}

func (pearlSample *PearlSample) Close() error {
	return nil
}

func (pearlSample *PearlSample) ingestRow(state *datura.Artifact) []float64 {
	channel := datura.Peek[string](state, "channel")

	switch channel {
	case "trade":
		return pearlSample.ingestTrade(state)
	case "ticker":
		return pearlSample.ingestTicker(state)
	}

	return nil
}

func (pearlSample *PearlSample) ingestTrade(state *datura.Artifact) []float64 {
	price := datura.Peek[float64](state, "data", 0, "price")
	quantity := datura.Peek[float64](state, "data", 0, "qty")
	side := datura.Peek[string](state, "data", 0, "side")

	if price <= 0 || quantity <= 0 {
		return nil
	}

	flow := quantity

	if side == "sell" {
		flow = -quantity
	}

	pearlSample.lastFlow = flow

	return []float64{
		pearlSample.lastMacro,
		pearlSample.lastLiquidity,
		flow,
		price,
	}
}

func (pearlSample *PearlSample) ingestTicker(state *datura.Artifact) []float64 {
	last := datura.Peek[float64](state, "data", 0, "last")
	changePct := datura.Peek[float64](state, "data", 0, "change_pct")
	bidQty := datura.Peek[float64](state, "data", 0, "bid_qty")
	askQty := datura.Peek[float64](state, "data", 0, "ask_qty")

	if last <= 0 {
		return nil
	}

	liquidity := bidQty + askQty

	if liquidity <= 0 {
		liquidity = pearlSample.lastLiquidity
	}

	pearlSample.lastMacro = changePct / 100
	pearlSample.lastLiquidity = liquidity

	return []float64{
		pearlSample.lastMacro,
		liquidity,
		pearlSample.lastFlow,
		last,
	}
}
