package equation

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

var ignitionEpoch = time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC)

// ignitionInput advances cumulative volume, price, and time on a steady clock so
// successive calls close equal-volume bars with a real return and intensity.
func ignitionInput(index int) IgnitionInput {
	return IgnitionInput{
		Symbol: "BTC/USD",
		Volume: 1000 + float64(index*20),
		Last:   100 + float64(index),
		Bid:    99.5 + float64(index),
		Ask:    100.5 + float64(index),
		At:     ignitionEpoch.Add(time.Duration(index) * time.Second),
	}
}

// warm drives enough steady observations to close several volume bars.
func warm(ignition *Ignition, count int) (IgnitionOutput, bool) {
	var output IgnitionOutput
	var ready bool

	for index := range count {
		output, ready, _, _ = ignition.Measure(ignitionInput(index))
	}

	return output, ready
}

func TestIgnition_Measure(testingTB *testing.T) {
	Convey("Given no baseline retention capacity", testingTB, func() {
		_, _, _, err := NewIgnition(0).Measure(ignitionInput(0))

		Convey("It should reject an unbounded calculator", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given steady volume-clock advances", testingTB, func() {
		ignition := NewIgnition(128)
		output, ready := warm(ignition, 12)

		Convey("It measures ignition on closed volume bars", func() {
			So(ready, ShouldBeTrue)
			So(output.RVOL, ShouldBeGreaterThan, 0)
			So(output.Precursor, ShouldBeGreaterThan, 0)
			So(output.Spread, ShouldBeGreaterThan, 0)
			So(output.Strength, ShouldBeGreaterThanOrEqualTo, 0)
		})
	})

	Convey("Given quote churn with no new executed volume", testingTB, func() {
		ignition := NewIgnition(128)
		warm(ignition, 12)

		// Quote-only ticks: price wanders, volume and time frozen, so no bar
		// closes. This is the live failure mode that used to zero every field.
		var output IgnitionOutput
		var ready bool

		for index := range 20 {
			output, ready, _, _ = ignition.Measure(IgnitionInput{
				Symbol: "BTC/USD",
				Volume: 1000 + float64(11*20), // unchanged cumulative volume
				Last:   111 + float64(index%2),
				Bid:    110.5,
				Ask:    111.5,
				At:     ignitionEpoch.Add(12 * time.Second),
			})
		}

		Convey("It keeps reporting the live spread instead of blanking", func() {
			So(ready, ShouldBeTrue)
			So(output.Spread, ShouldEqual, 1.0)
		})

		Convey("It never poisons the move baseline with quote-churn zeros", func() {
			window := ignition.windows["BTC/USD"]
			for _, sample := range window.returns {
				So(sample, ShouldBeGreaterThan, 0)
			}
		})
	})

	Convey("Given a calm bar after history is ready", testingTB, func() {
		ignition := NewIgnition(128)
		warm(ignition, 12)

		// A flat-price bar with real executed volume: intensity stays valid but
		// the upward precursor is zero, so ignition collapses without erroring.
		output, ready, _, err := ignition.Measure(IgnitionInput{
			Symbol: "BTC/USD",
			Volume: 1000 + float64(12*20),
			Last:   111,
			Bid:    110.5,
			Ask:    111.5,
			At:     ignitionEpoch.Add(13 * time.Second),
		})

		Convey("It emits a live spread with a zero ignition score", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(output.Spread, ShouldEqual, 1.0)
			So(output.Ignition, ShouldEqual, 0)
		})
	})

	Convey("Given a pump after a sustained decline", testingTB, func() {
		ignition := NewIgnition(128)
		warm(ignition, 12)

		for index := range 16 {
			_, _, _, err := ignition.Measure(IgnitionInput{
				Symbol: "BTC/USD",
				Volume: 1000 + float64((12+index)*20),
				Last:   111 - float64(index+1),
				Bid:    110.5 - float64(index+1),
				Ask:    111.5 - float64(index+1),
				At:     ignitionEpoch.Add(time.Duration(12+index) * time.Second),
			})
			So(err, ShouldBeNil)
		}

		output, ready, _, err := ignition.Measure(IgnitionInput{
			Symbol: "BTC/USD",
			Volume: 1000 + float64(28*20),
			Last:   100,
			Bid:    99.5,
			Ask:    100.5,
			At:     ignitionEpoch.Add(28 * time.Second),
		})

		Convey("It should retain a usable upward-move scale", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(output.Precursor, ShouldBeGreaterThan, 0)
			So(output.Ignition, ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given a bounded retention capacity", testingTB, func() {
		const capacity = 16
		ignition := NewIgnition(capacity)

		for index := range 60 {
			_, _, _, err := ignition.Measure(ignitionInput(index))
			So(err, ShouldBeNil)
		}

		Convey("It never grows a baseline past capacity", func() {
			window := ignition.windows["BTC/USD"]
			So(len(window.rates), ShouldBeLessThanOrEqualTo, capacity)
			So(len(window.returns), ShouldBeLessThanOrEqualTo, capacity)
			So(len(window.precursors), ShouldBeLessThanOrEqualTo, capacity)
			So(len(window.spreads), ShouldBeLessThanOrEqualTo, capacity)
		})
	})
}

func BenchmarkIgnition_Measure(benchmark *testing.B) {
	ignition := NewIgnition(128)

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		for index := range 8 {
			_, _, _, _ = ignition.Measure(ignitionInput(index))
		}
	}
}
