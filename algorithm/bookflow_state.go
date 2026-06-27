package algorithm

import (
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/bytedance/sonic"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
	"github.com/theapemachine/nomagique/probability"
	"gonum.org/v1/gonum/stat"
)

const (
	SideBid byte = 'b'
	SideAsk byte = 'a'
)

/*
CancelFillRatio returns cancel volume divided by matched fill volume.
*/
func CancelFillRatio(cancel, fill float64) (float64, error) {
	if cancel <= 0 || fill <= 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"algorithm: cancel and fill must be positive",
			nil,
		))
	}

	return cancel / fill, nil
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
) (float64, error) {
	if sizeThreshold <= 0 || qty < sizeThreshold {
		return 0, fmt.Errorf("algorithm: toxic cancel qty below size threshold")
	}

	if proximityPct <= 0 || distancePct > proximityPct || maxAge <= 0 || age > maxAge {
		return 0, fmt.Errorf("algorithm: toxic cancel proximity or age gate failed")
	}

	sizeEvidence, err := probability.CompetitionMargin(qty-sizeThreshold, sizeThreshold)

	if err != nil {
		return 0, err
	}

	proximityEvidence, err := probability.CompetitionMargin(proximityPct-distancePct, proximityPct)

	if err != nil {
		return 0, err
	}

	ageEvidence, err := probability.CompetitionMargin(float64(maxAge-age), float64(maxAge))

	if err != nil {
		return 0, err
	}

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
) (float64, error) {
	if ratio <= 0 || sizeThreshold <= 0 || addVol < sizeThreshold {
		return 0, fmt.Errorf("algorithm: toxic churn ratio or size gate failed")
	}

	if churnGate > 0 && ratio <= churnGate {
		return 0, fmt.Errorf("algorithm: toxic churn ratio or size gate failed")
	}

	if proximityPct <= 0 || distancePct > proximityPct {
		return 0, fmt.Errorf("algorithm: toxic churn proximity gate failed")
	}

	ratioEvidence, err := probability.MagnitudeMargin(ratio)
	if churnGate > 0 {
		ratioEvidence, err = probability.CompetitionMargin(ratio-churnGate, churnGate)
	}

	if err != nil {
		return 0, err
	}

	sizeEvidence, err := probability.CompetitionMargin(addVol-sizeThreshold, sizeThreshold)

	if err != nil {
		return 0, err
	}

	proximityEvidence, err := probability.CompetitionMargin(proximityPct-distancePct, proximityPct)

	if err != nil {
		return 0, err
	}

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
}

/*
NewGateQuantile wires a gate stage from a config artifact.
*/
func NewGateQuantile(artifact *datura.Artifact) *GateQuantile {
	artifact.Poke([]float64{}, "history")

	return &GateQuantile{
		artifact: artifact,
	}
}

func (gate *GateQuantile) Write(payload []byte) (int, error) {
	gate.artifact.WithPayload(payload)
	return len(payload), nil
}

func (gate *GateQuantile) Read(payload []byte) (int, error) {
	state := datura.Acquire("gate-quantile-state", datura.APPJSON)

	if _, err := state.Unpack(gate.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"algorithm: state write failed",
			err,
		))
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

	gateValue := gate.value(percentile)

	state.MergeOutput("value", gateValue)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")

	return state.PackInto(payload)
}

/*
value returns the configured quantile over persisted history.
percentileOverride replaces the config percentile when positive.
*/
func (gate *GateQuantile) value(percentileOverride float64) float64 {
	percentile := percentileOverride

	if percentile <= 0 {
		percentile = datura.Peek[float64](gate.artifact, "percentile")
	}

	minSamples := int(datura.Peek[float64](gate.artifact, "minSamples"))

	if minSamples < 1 {
		minSamples = 3
	}

	history := datura.Peek[[]float64](gate.artifact, "history")

	if len(history) < minSamples {
		return 0
	}

	value, err := gateSampleQuantile(percentile, history)

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"gate-quantile: quantile failed",
			err,
		))

		return 0
	}

	return value
}

/*
observe records one sample through the wire bus and returns the updated quantile.
*/
func (gate *GateQuantile) observe(sample float64, percentileOverride float64) float64 {
	fields := datura.Map[float64]{"sample": sample}

	if percentileOverride > 0 {
		fields["percentile"] = percentileOverride
	}

	payload := errnie.Does(func() ([]byte, error) {
		return sonic.Marshal(fields)
	}).Or(func(err error) {
		errnie.Error(errnie.Err(errnie.IO, "gate-quantile: marshal observe payload", err))
	}).Value()

	frame := datura.Acquire("gate-frame", datura.APPJSON)

	if frame.WithPayload(payload) == nil {
		frame.Release()

		return 0
	}

	packed := frame.Pack()

	frame.Release()

	if len(packed) == 0 {
		return 0
	}

	if _, err := gate.Write(packed); err != nil {
		return 0
	}

	bufferSize := max(len(packed)*2, len(packed)+1)
	buffer := make([]byte, bufferSize)
	readCount, err := gate.Read(buffer)

	for err == io.ErrShortBuffer {
		bufferSize *= 2
		buffer = make([]byte, bufferSize)
		readCount, err = gate.Read(buffer)
	}

	if err != nil && err != io.EOF && err != io.ErrShortBuffer {
		return 0
	}

	outbound := datura.Acquire("gate-read", datura.APPJSON)
	_, _ = outbound.Unpack(buffer[:readCount])
	value := datura.Peek[float64](outbound, "output", "value")
	outbound.Release()

	return value
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

	span, err := gateDistinctSpan(values)

	if err != nil {
		errnie.Error(errnie.Err(
			errnie.Validation,
			"gate-quantile: span failed",
			err,
		))

		return minSamples
	}

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

func gateSampleQuantile(percentile float64, values []float64) (float64, error) {
	if len(values) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"gate-quantile: values required",
			nil,
		))
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	for _, value := range sorted {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"gate-quantile: sample is non-finite",
				nil,
			))
		}
	}

	return stat.Quantile(percentile, stat.LinInterp, sorted, nil), nil
}

func gateDistinctSpan(values []float64) (float64, error) {
	if len(values) == 0 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"gate-quantile: span values required",
			nil,
		))
	}

	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	distinct := 1

	for index := 1; index < len(sorted); index++ {
		if sorted[index] == sorted[index-1] {
			continue
		}

		distinct++
	}

	if distinct <= 1 {
		return 0, nil
	}

	return float64(distinct), nil
}

var _ io.ReadWriteCloser = (*GateQuantile)(nil)
