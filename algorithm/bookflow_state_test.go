package algorithm

import (
	"math"
	"testing"
	"time"

	"github.com/theapemachine/nomagique/statistic"
	. "github.com/smartystreets/goconvey/convey"
)

func TestToxicCancelEvidence(testingTB *testing.T) {
	cases := []struct {
		name      string
		qty       float64
		threshold float64
		distance  float64
		proximity float64
		age       time.Duration
		maxAge    time.Duration
		wantZero  bool
	}{
		{name: "below size", qty: 1, threshold: 2, distance: 0.001, proximity: 0.01, age: time.Second, maxAge: 10 * time.Second, wantZero: true},
		{name: "too far", qty: 10, threshold: 2, distance: 0.05, proximity: 0.01, age: time.Second, maxAge: 10 * time.Second, wantZero: true},
		{name: "too old", qty: 10, threshold: 2, distance: 0.001, proximity: 0.01, age: 20 * time.Second, maxAge: 10 * time.Second, wantZero: true},
		{name: "valid", qty: 10, threshold: 2, distance: 0.001, proximity: 0.01, age: time.Second, maxAge: 10 * time.Second, wantZero: false},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given ToxicCancelEvidence "+testCase.name, testingTB, func() {
			evidence := ToxicCancelEvidence(
				testCase.qty,
				testCase.threshold,
				testCase.distance,
				testCase.proximity,
				testCase.age,
				testCase.maxAge,
			)

			if testCase.wantZero {
				Convey("It should return zero", func() {
					So(evidence, ShouldEqual, 0)
				})

				return
			}

			Convey("It should return positive evidence", func() {
				So(evidence, ShouldBeGreaterThan, 0)
				So(evidence, ShouldBeLessThanOrEqualTo, 1)
			})
		})
	}
}

func TestToxicChurnEvidence(testingTB *testing.T) {
	cases := []struct {
		name      string
		ratio     float64
		gate      float64
		addVol    float64
		threshold float64
		distance  float64
		proximity float64
		wantZero  bool
	}{
		{name: "below churn gate", ratio: 1, gate: 2, addVol: 10, threshold: 2, distance: 0.001, proximity: 0.01, wantZero: true},
		{name: "small add", ratio: 5, gate: 2, addVol: 1, threshold: 2, distance: 0.001, proximity: 0.01, wantZero: true},
		{name: "too far", ratio: 5, gate: 2, addVol: 10, threshold: 2, distance: 0.05, proximity: 0.01, wantZero: true},
		{name: "valid", ratio: 5, gate: 2, addVol: 10, threshold: 2, distance: 0.001, proximity: 0.01, wantZero: false},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given ToxicChurnEvidence "+testCase.name, testingTB, func() {
			evidence := ToxicChurnEvidence(
				testCase.ratio,
				testCase.gate,
				testCase.addVol,
				testCase.threshold,
				testCase.distance,
				testCase.proximity,
			)

			if testCase.wantZero {
				Convey("It should return zero", func() {
					So(evidence, ShouldEqual, 0)
				})

				return
			}

			Convey("It should return positive evidence", func() {
				So(evidence, ShouldBeGreaterThan, 0)
			})
		})
	}
}

func TestSideFlowLedger(testingTB *testing.T) {
	Convey("Given bid depth updates", testingTB, func() {
		ledger := SideFlowLedger{}
		ledger.AddDepth(SideBid, 10)
		ledger.AddDepth(SideBid, -3)

		Convey("It should floor depth at zero", func() {
			So(ledger.SideDepth(SideBid), ShouldEqual, 7)
		})
	})

	Convey("Given zero smoothing alpha", testingTB, func() {
		ledger := SideFlowLedger{}
		ledger.ApplyFlow(SideAsk, 5, 2, 0)

		Convey("It should replace flow outright", func() {
			So(ledger.FillAsk, ShouldEqual, 5)
			So(ledger.CancelAsk, ShouldEqual, 2)
		})
	})

	Convey("Given positive smoothing alpha", testingTB, func() {
		ledger := SideFlowLedger{FillBid: 10, CancelBid: 4}
		ledger.ApplyFlow(SideBid, 20, 8, 0.5)

		Convey("It should exponentially smooth flows", func() {
			So(ledger.FillBid, ShouldEqual, 15)
			So(ledger.CancelBid, ShouldEqual, 6)
		})
	})

	Convey("Given snapshot", testingTB, func() {
		ledger := SideFlowLedger{
			BidDepth: 1, AskDepth: 2,
			FillBid: 3, CancelBid: 4,
			FillAsk: 5, CancelAsk: 6,
		}
		cancelBid, fillBid, cancelAsk, fillAsk, bidDepth, askDepth := ledger.Snapshot()

		Convey("It should export all fields", func() {
			So(cancelBid, ShouldEqual, 4)
			So(fillBid, ShouldEqual, 3)
			So(cancelAsk, ShouldEqual, 6)
			So(fillAsk, ShouldEqual, 5)
			So(bidDepth, ShouldEqual, 1)
			So(askDepth, ShouldEqual, 2)
		})
	})
}

func TestCancelFillRatio(testingTB *testing.T) {
	Convey("Given non-positive inputs", testingTB, func() {
		Convey("It should return zero", func() {
			So(CancelFillRatio(0, 5), ShouldEqual, 0)
			So(CancelFillRatio(5, 0), ShouldEqual, 0)
		})
	})

	Convey("Given positive cancel and fill", testingTB, func() {
		Convey("It should divide cancel by fill", func() {
			So(CancelFillRatio(6, 3), ShouldEqual, 2)
		})
	})
}

func TestBookGates(testingTB *testing.T) {
	Convey("Given insufficient ring history", testingTB, func() {
		gates := NewBookGates()

		Convey("It should return default gates", func() {
			So(gates.ChurnRatioGate(), ShouldEqual, 0)
			So(gates.FillCoverageGate(), ShouldEqual, 1)
		})
	})

	Convey("Given populated observation rings", testingTB, func() {
		gates := NewBookGates()
		for _, value := range []float64{1, 2, 3, 4} {
			gates.ChurnRatios.Observe(value)
			gates.FillMatchRatios.Observe(value * 0.1)
			gates.CancelQtys.Observe(value * 10)
			gates.LevelSizeFracs.Observe(value * 0.05)
			gates.VacuumRatios.Observe(value * 0.2)
		}

		Convey("It should derive quantile gates", func() {
			So(gates.ChurnRatioGate(), ShouldBeGreaterThan, 0)
			So(gates.FillCoverageGate(), ShouldBeGreaterThan, 0)
			So(gates.LargeBlockQtyThreshold(100, 0), ShouldBeGreaterThan, 0)
			So(gates.VacuumStrengthLimit(0.5, 0), ShouldBeGreaterThan, 0)
			So(gates.SupportRatioGate(0.5), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given zero side depth", testingTB, func() {
		gates := NewBookGates()

		Convey("It should return infinite block threshold", func() {
			threshold := gates.LargeBlockQtyThreshold(0, 5)
			So(math.IsInf(threshold, 1), ShouldBeTrue)
		})
	})

	Convey("Given median level fallback", testingTB, func() {
		gates := NewBookGates()

		Convey("It should use median level quantity", func() {
			So(gates.LargeBlockQtyThreshold(100, 7), ShouldEqual, 7)
		})
	})
}

func TestObservationRingAdversarial(testingTB *testing.T) {
	Convey("Given non-positive observations", testingTB, func() {
		ring := statistic.NewObservationRing()
		ring.Observe(0)
		ring.Observe(-1)

		Convey("It should ignore invalid samples", func() {
			So(ring.Len(), ShouldEqual, 0)
		})
	})
}
