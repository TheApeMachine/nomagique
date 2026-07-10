package statistic

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestRollingZScore_Measure(testingTB *testing.T) {
	Convey("Given the first sample with nothing preceding it", testingTB, func() {
		rollingZScore := NewRollingZScore()
		output, err := rollingZScore.Measure(TimedSample{
			Value: 10,
			At:    time.Unix(1, 0),
		})

		So(err, ShouldBeNil)

		Convey("It should report the reflexive zero score, ready", func() {
			So(output.Ready, ShouldBeTrue)
			So(output.Value, ShouldEqual, 0)
			So(output.Count, ShouldEqual, 1)
		})
	})

	Convey("Given a series that regresses in time", testingTB, func() {
		rollingZScore := NewRollingZScore()
		_, err := rollingZScore.Measure(TimedSample{Value: 10, At: time.Unix(2, 0)})
		So(err, ShouldBeNil)

		_, err = rollingZScore.Measure(TimedSample{Value: 10, At: time.Unix(1, 0)})

		Convey("It should reject the regression", func() {
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a long-running stream with no explicit long-window hint", testingTB, func() {
		rollingZScore := NewRollingZScore()

		for index := range 200 {
			_, err := rollingZScore.Measure(TimedSample{
				Value: float64(index % 7),
				At:    time.Unix(int64(index+1), 0),
			})

			So(err, ShouldBeNil)
		}

		// ResolveWindowSet resolves a window sized to whatever history
		// already exists, so without an explicit LongHint retained
		// history tracks input length one-for-one — the same
		// characteristic MeanMedianRatio has when unconfigured. Callers
		// feeding an indefinitely long single series must set LongHint
		// to bound retained history and per-sample recompute cost.
		Convey("It should retain the full series, matching MeanMedianRatio's unconfigured behavior", func() {
			So(len(rollingZScore.samples["default"]), ShouldEqual, 200)
		})
	})

	Convey("Given an explicit long-window hint", testingTB, func() {
		rollingZScore := NewRollingZScore(RollingZScoreConfig{LongHint: 3})

		for index := range 10 {
			_, err := rollingZScore.Measure(TimedSample{
				Value: float64(index),
				At:    time.Unix(int64(index+1), 0),
			})

			So(err, ShouldBeNil)
		}

		Convey("It should trim retained history to the configured window", func() {
			So(len(rollingZScore.samples["default"]), ShouldEqual, 3)
		})
	})
}

func BenchmarkRollingZScoreMeasure(benchmark *testing.B) {
	rollingZScore := NewRollingZScore()
	at := time.Unix(1, 0)

	benchmark.ReportAllocs()

	for benchmark.Loop() {
		at = at.Add(time.Second)

		if _, err := rollingZScore.Measure(TimedSample{Value: 10, At: at}); err != nil {
			benchmark.Fatal(err)
		}
	}
}
