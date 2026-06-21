package algorithm

import (
	"io"
	"math"
	"time"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/statistic"
)

const gateReadBufferSize = 65536

const (
	SideBid byte = 'b'
	SideAsk byte = 'a'
)

/*
CancelFillRatio returns cancel volume divided by matched fill volume.
*/
func CancelFillRatio(cancel, fill float64) float64 {
	if cancel <= 0 || fill <= 0 {
		return 0
	}

	return cancel / fill
}

/*
ToxicCancelEvidence scores a large near-touch cancel relative to observed gates.
*/
func ToxicCancelEvidence(
	qty float64,
	sizeThreshold float64,
	distancePct float64,
	proximityPct float64,
	age time.Duration,
	maxAge time.Duration,
) float64 {
	if sizeThreshold <= 0 || qty < sizeThreshold {
		return 0
	}

	if proximityPct <= 0 || distancePct > proximityPct || maxAge <= 0 || age > maxAge {
		return 0
	}

	sizeEvidence := probability.CompetitionMargin(qty-sizeThreshold, sizeThreshold)
	proximityEvidence := probability.CompetitionMargin(proximityPct-distancePct, proximityPct)
	ageEvidence := probability.CompetitionMargin(float64(maxAge-age), float64(maxAge))

	return probability.EvidenceGeomean(sizeEvidence, proximityEvidence, ageEvidence)
}

/*
ToxicChurnEvidence scores flash churn near touch relative to observed gates.
*/
func ToxicChurnEvidence(
	ratio float64,
	churnGate float64,
	addVol float64,
	sizeThreshold float64,
	distancePct float64,
	proximityPct float64,
) float64 {
	if ratio <= churnGate || sizeThreshold <= 0 || addVol < sizeThreshold {
		return 0
	}

	if proximityPct <= 0 || distancePct > proximityPct {
		return 0
	}

	ratioEvidence := probability.CompetitionMargin(ratio-churnGate, churnGate)
	sizeEvidence := probability.CompetitionMargin(addVol-sizeThreshold, sizeThreshold)
	proximityEvidence := probability.CompetitionMargin(proximityPct-distancePct, proximityPct)

	return probability.EvidenceGeomean(ratioEvidence, sizeEvidence, proximityEvidence)
}

/*
SideFlowLedger tracks per-side depth and smoothed cancel/fill flow.
*/
type SideFlowLedger struct {
	BidDepth  float64
	AskDepth  float64
	FillBid   float64
	CancelBid float64
	FillAsk   float64
	CancelAsk float64
}

func (ledger *SideFlowLedger) AddDepth(side byte, delta float64) {
	if side == SideBid {
		ledger.BidDepth = maxFloat(0, ledger.BidDepth+delta)

		return
	}

	ledger.AskDepth = maxFloat(0, ledger.AskDepth+delta)
}

func (ledger *SideFlowLedger) SideDepth(side byte) float64 {
	if side == SideBid {
		return ledger.BidDepth
	}

	return ledger.AskDepth
}

func (ledger *SideFlowLedger) ApplyFlow(side byte, fill, cancel, alpha float64) {
	if alpha <= 0 {
		if side == SideBid {
			ledger.FillBid = fill
			ledger.CancelBid = cancel

			return
		}

		ledger.FillAsk = fill
		ledger.CancelAsk = cancel

		return
	}

	if side == SideBid {
		ledger.FillBid += alpha * (fill - ledger.FillBid)
		ledger.CancelBid += alpha * (cancel - ledger.CancelBid)

		return
	}

	ledger.FillAsk += alpha * (fill - ledger.FillAsk)
	ledger.CancelAsk += alpha * (cancel - ledger.CancelAsk)
}

func (ledger *SideFlowLedger) Snapshot() (
	cancelBid, fillBid, cancelAsk, fillAsk, bidDepth, askDepth float64,
) {
	return ledger.CancelBid, ledger.FillBid, ledger.CancelAsk, ledger.FillAsk, ledger.BidDepth, ledger.AskDepth
}

func maxFloat(left, right float64) float64 {
	if left > right {
		return left
	}

	return right
}

/*
GateQuantile retains observations on the config artifact and emits a quantile
gate under output.value. Config attributes: percentile, minSamples.
*/
type GateQuantile struct {
	artifact *datura.Artifact
	bytes    []byte
}

/*
NewGateQuantile wires a gate stage from a config artifact.
*/
func NewGateQuantile(artifact *datura.Artifact) *GateQuantile {
	artifact.Inspect("algorithm", "gate-quantile", "NewGateQuantile()")

	return &GateQuantile{
		artifact: artifact,
	}
}

func (gate *GateQuantile) Write(payload []byte) (int, error) {
	gate.bytes = append(gate.bytes[:0], payload...)

	return len(payload), nil
}

func (gate *GateQuantile) Read(payload []byte) (int, error) {
	state := datura.Acquire("gate-quantile-state", datura.APPJSON)

	if _, err := state.Write(gate.bytes); err != nil {
		state.Release()

		return 0, err
	}

	defer state.Release()

	sample := datura.Peek[float64](state, "sample")
	percentile := datura.Peek[float64](state, "percentile")

	if percentile <= 0 {
		percentile = datura.Peek[float64](gate.artifact, "percentile")
	}

	minSamples := int(datura.Peek[float64](gate.artifact, "minSamples"))

	if minSamples < 1 {
		minSamples = 3
	}

	if sample > 0 && !math.IsNaN(sample) && !math.IsInf(sample, 0) {
		history := datura.Peek[[]float64](gate.artifact, "history")
		history = append(history, sample)

		capacity := gateHistoryCapacity(history, minSamples)

		if len(history) > capacity {
			history = history[len(history)-capacity:]
		}

		gate.artifact.Poke(history, "history")
	}

	history := datura.Peek[[]float64](gate.artifact, "history")
	gateValue := 0.0

	if len(history) >= minSamples {
		gateValue = statistic.QuantileOf(percentile, history)
	}

	outState := datura.Acquire("gate-quantile-out", datura.APPJSON)
	outState.MergeOutput("value", gateValue)
	outState.Merge("root", "output")
	outState.Merge("inputs", []string{"value"})

	return outState.Read(payload)
}

func (gate *GateQuantile) Close() error {
	return nil
}

func gateHistoryCapacity(values []float64, minSamples int) int {
	if len(values) == 0 {
		return 1
	}

	if len(values) < 3 {
		return len(values) + 1
	}

	span := statistic.SpanOf(values)

	if span <= 0 {
		if minSamples > 3 {
			return minSamples
		}

		return 3
	}

	capacity := int(span) + 1

	if capacity < minSamples {
		return minSamples
	}

	return capacity
}

func largeBlockQtyThreshold(
	sideDepth float64,
	medianLevelQty float64,
	cancelQtyGate float64,
	levelSizeFracGate float64,
	cancelQtyReady bool,
	levelSizeFracReady bool,
) float64 {
	if sideDepth <= 0 {
		return mathInf(1)
	}

	if cancelQtyReady {
		return cancelQtyGate
	}

	if levelSizeFracReady {
		return levelSizeFracGate * sideDepth
	}

	if medianLevelQty > 0 {
		return medianLevelQty
	}

	return sideDepth / maxFloat(1, math.Sqrt(sideDepth))
}

func vacuumStrengthLimit(
	threshold float64,
	peakVacuumRatio float64,
	vacuumPeak float64,
	vacuumReady bool,
) float64 {
	if threshold <= 0 {
		return 1
	}

	if vacuumReady {
		return maxFloat(vacuumPeak/threshold, vacuumPeak)
	}

	if peakVacuumRatio > 0 {
		return peakVacuumRatio / threshold
	}

	return 1
}

func supportRatioGate(threshold float64, vacuumLow float64, vacuumReady bool) float64 {
	if threshold <= 0 || !vacuumReady {
		return 0
	}

	return vacuumLow / threshold
}

func gateReady(config *datura.Artifact) bool {
	return len(datura.Peek[[]float64](config, "history")) >= 3
}

func mathInf(sign int) float64 {
	return math.Inf(sign)
}

var _ io.ReadWriteCloser = (*GateQuantile)(nil)
