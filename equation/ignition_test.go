package equation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func ignitionInput(index int) IgnitionInput {
	return IgnitionInput{
		Symbol: "BTC/USD",
		Volume: 1000 + float64(index*20),
		Last:   100 + float64(index),
		Bid:    99.5 + float64(index),
		Ask:    100.5 + float64(index),
	}
}

func TestIgnition_Measure(testingTB *testing.T) {
	Convey("Given direct ticker inputs", testingTB, func() {
		ignition := NewIgnition()
		var output IgnitionOutput
		var ready bool
		var err error

		for index := range 8 {
			output, ready, _, err = ignition.Measure(ignitionInput(index))
		}

		Convey("It measures ignition without artifact transport", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(output.RVOL, ShouldBeGreaterThan, 0)
			So(output.Precursor, ShouldBeGreaterThan, 0)
			So(output.Spread, ShouldBeGreaterThan, 0)
			So(output.Strength, ShouldBeGreaterThanOrEqualTo, 0)
		})
	})

	Convey("Given a zero-lift update after history is ready", testingTB, func() {
		ignition := NewIgnition()

		for index := range 8 {
			_, _, _, err := ignition.Measure(ignitionInput(index))
			So(err, ShouldBeNil)
		}

		output, ready, _, err := ignition.Measure(IgnitionInput{
			Symbol: "BTC/USD",
			Volume: 1140,
			Last:   107,
			Bid:    106.5,
			Ask:    107.5,
		})

		Convey("It emits a zero-score observation instead of an error", func() {
			So(err, ShouldBeNil)
			So(ready, ShouldBeTrue)
			So(output.Strength, ShouldBeGreaterThanOrEqualTo, 0)
		})
	})
}

func BenchmarkIgnition_Measure(benchmark *testing.B) {
	ignition := NewIgnition()

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		for index := range 8 {
			_, _, _, _ = ignition.Measure(ignitionInput(index))
		}
	}
}
