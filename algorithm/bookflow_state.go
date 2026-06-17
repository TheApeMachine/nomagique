package algorithm

import (
	"math"
	"time"

	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/statistic"
)

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
	BidDepth   float64
	AskDepth   float64
	FillBid    float64
	CancelBid  float64
	FillAsk    float64
	CancelAsk  float64
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
BookGates derives classification thresholds from observation rings.
*/
type BookGates struct {
	ChurnRatios     *statistic.ObservationRing
	FillMatchRatios *statistic.ObservationRing
	CancelQtys      *statistic.ObservationRing
	LevelSizeFracs  *statistic.ObservationRing
	VacuumRatios    *statistic.ObservationRing
}

func NewBookGates() *BookGates {
	return &BookGates{
		ChurnRatios:     statistic.NewObservationRing(),
		FillMatchRatios: statistic.NewObservationRing(),
		CancelQtys:      statistic.NewObservationRing(),
		LevelSizeFracs:  statistic.NewObservationRing(),
		VacuumRatios:    statistic.NewObservationRing(),
	}
}

func (gates *BookGates) ChurnRatioGate() float64 {
	if gates.ChurnRatios.Len() >= 3 {
		return gates.ChurnRatios.Quantile(0.75)
	}

	return 0
}

func (gates *BookGates) FillCoverageGate() float64 {
	if gates.FillMatchRatios.Len() >= 3 {
		return gates.FillMatchRatios.Quantile(0.5)
	}

	return 1
}

func (gates *BookGates) LargeBlockQtyThreshold(sideDepth float64, medianLevelQty float64) float64 {
	if sideDepth <= 0 {
		return mathInf(1)
	}

	if gates.CancelQtys.Len() >= 3 {
		return gates.CancelQtys.Quantile(0.5)
	}

	if gates.LevelSizeFracs.Len() >= 3 {
		frac := gates.LevelSizeFracs.Quantile(0.75)

		return frac * sideDepth
	}

	if medianLevelQty > 0 {
		return medianLevelQty
	}

	return sideDepth / maxFloat(1, math.Sqrt(sideDepth))
}

func (gates *BookGates) VacuumStrengthLimit(threshold, peakVacuumRatio float64) float64 {
	if threshold <= 0 {
		return 1
	}

	if gates.VacuumRatios.Len() >= 3 {
		peak := gates.VacuumRatios.Quantile(0.9)

		return maxFloat(peak/threshold, peak)
	}

	if peakVacuumRatio > 0 {
		return peakVacuumRatio / threshold
	}

	return 1
}

func (gates *BookGates) SupportRatioGate(threshold float64) float64 {
	if threshold <= 0 || gates.VacuumRatios.Len() < 3 {
		return 0
	}

	low := gates.VacuumRatios.Quantile(0.25)

	return low / threshold
}

func mathInf(sign int) float64 {
	return math.Inf(sign)
}
