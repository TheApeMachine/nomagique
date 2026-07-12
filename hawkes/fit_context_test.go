package hawkes

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewFitContext(testingTB *testing.T) {
	Convey("Given fewer than two marked events", testingTB, func() {
		start := time.Unix(100, 0)
		stream := NewArrivalStream([]time.Time{start}, nil)
		context, ok := NewFitContext(stream, start.Add(time.Second))

		Convey("It should reject the stream", func() {
			So(ok, ShouldBeFalse)
			So(context.TotalEvents, ShouldEqual, 0)
		})
	})

	Convey("Given a valid bivariate stream", testingTB, func() {
		start := time.Unix(200, 0)
		stream := NewArrivalStream(
			[]time.Time{start, start.Add(time.Second), start.Add(2 * time.Second), start.Add(3 * time.Second)},
			[]time.Time{start.Add(500 * time.Millisecond), start.Add(1500 * time.Millisecond), start.Add(2500 * time.Millisecond)},
		)
		horizon := start.Add(4 * time.Second)
		context, ok := NewFitContext(stream, horizon)

		Convey("It should derive positive fit bounds", func() {
			So(ok, ShouldBeTrue)
			So(context.SpanSec, ShouldBeGreaterThan, 0)
			So(context.MedianGapSec, ShouldBeGreaterThan, 0)
			So(context.TotalEvents, ShouldEqual, 7)
			So(context.EventsX, ShouldEqual, 4)
			So(context.EventsY, ShouldEqual, 3)
			So(len(context.BetaCandidates), ShouldBeGreaterThan, 0)
			So(len(context.MuXFactors), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given horizon before first event", testingTB, func() {
		start := time.Unix(300, 0)
		stream := NewArrivalStream(
			[]time.Time{start, start.Add(time.Second)},
			[]time.Time{start.Add(500 * time.Millisecond)},
		)
		context, ok := NewFitContext(stream, start.Add(-time.Second))

		Convey("It should reject non-positive span", func() {
			So(ok, ShouldBeFalse)
			So(context.SpanSec, ShouldEqual, 0)
		})
	})
}

func TestNewObservationContext(testingTB *testing.T) {
	Convey("Given the same valid arrival stream as a complete fit context", testingTB, func() {
		start := time.Unix(350, 0)
		stream := NewArrivalStream(
			[]time.Time{start, start.Add(time.Second), start.Add(3 * time.Second)},
			[]time.Time{start.Add(500 * time.Millisecond), start.Add(2 * time.Second)},
		)
		horizon := start.Add(4 * time.Second)
		observation, observationOK := NewObservationContext(stream, horizon)
		complete, completeOK := NewFitContext(stream, horizon)

		Convey("It should preserve every observed statistic without allocating search grids", func() {
			So(observationOK, ShouldBeTrue)
			So(completeOK, ShouldBeTrue)
			So(observation.SpanSec, ShouldEqual, complete.SpanSec)
			So(observation.MedianGapSec, ShouldEqual, complete.MedianGapSec)
			So(observation.GapLowerSec, ShouldEqual, complete.GapLowerSec)
			So(observation.GapUpperSec, ShouldEqual, complete.GapUpperSec)
			So(observation.GapCV, ShouldEqual, complete.GapCV)
			So(observation.TotalEvents, ShouldEqual, complete.TotalEvents)
			So(observation.MinFitEvents, ShouldEqual, complete.MinFitEvents)
			So(observation.TradeWindow, ShouldEqual, complete.TradeWindow)
			So(observation.BetaCandidates, ShouldBeEmpty)
			So(complete.BetaCandidates, ShouldNotBeEmpty)
		})
	})
}

func TestFitContext_EnoughEvents(testingTB *testing.T) {
	Convey("Given fit context minima", testingTB, func() {
		start := time.Unix(400, 0)
		buyTimes := make([]time.Time, 20)
		sellTimes := make([]time.Time, 20)

		for index := range buyTimes {
			buyTimes[index] = start.Add(time.Duration(index) * time.Second)
			sellTimes[index] = start.Add(time.Duration(index)*time.Second + 500*time.Millisecond)
		}

		stream := NewArrivalStream(buyTimes, sellTimes)
		context, ok := NewFitContext(stream, start.Add(20*time.Second))

		So(ok, ShouldBeTrue)

		Convey("It should accept a balanced stream", func() {
			So(context.EnoughEvents(stream), ShouldBeTrue)
		})

		Convey("It should reject a buy-only stream below minima", func() {
			buyOnly := NewArrivalStream(buyTimes[:3], nil)

			So(context.EnoughEvents(buyOnly), ShouldBeFalse)
		})

		Convey("It should accept a single-side stream at minFitEvents", func() {
			singleSideBuyTimes := make([]time.Time, context.MinFitEvents)

			for index := range singleSideBuyTimes {
				singleSideBuyTimes[index] = start.Add(time.Duration(index) * time.Second)
			}

			singleSide := NewArrivalStream(singleSideBuyTimes, nil)

			So(context.EnoughEvents(singleSide), ShouldBeTrue)
		})
	})
}
