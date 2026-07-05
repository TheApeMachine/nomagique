package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/correlation"
)

var barInterval = 5 * time.Minute

func TestLagEvaluateFollowerSync(testingTB *testing.T) {
	Convey("Given aligned follower correlation before anchor move gate warms", testingTB, func() {
		lag := NewLag()
		outcome, err := lag.Measure(LagInput{
			Price:       100,
			ContempOK:   true,
			ContempCorr: 0.9,
			SampleCount: 16,
		})

		So(err, ShouldBeNil)

		Convey("It should classify synchronized drift", func() {
			So(outcome.Eligible, ShouldBeTrue)
			So(outcome.Category, ShouldEqual, 2)
		})
	})
}

func TestLagEvaluateAnchorStall(testingTB *testing.T) {
	Convey("Given a warmed flat anchor path", testingTB, func() {
		lag := NewLag()
		outcome, err := lag.Measure(LagInput{
			IsAnchor:    true,
			Price:       50000,
			MoveReady:   true,
			StallMargin: 0.6,
		})

		So(err, ShouldBeNil)

		Convey("It should classify anchor stall", func() {
			So(outcome.Category, ShouldEqual, 4)
			So(outcome.Strength, ShouldBeGreaterThan, 0)
		})
	})
}

func TestLagInputFromFields(testingTB *testing.T) {
	Convey("Given fewer than eleven lag feature fields", testingTB, func() {
		_, err := LagInputFromFields([]float64{0, 100, 0, 0, 0})

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestCrossLagScore(testingTB *testing.T) {
	Convey("Given a follower that leads the anchor", testingTB, func() {
		start := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
		minSamples, err := lagMinSamples()
		So(err, ShouldBeNil)
		maxBars, err := lagMaxBars()
		So(err, ShouldBeNil)
		sampleCount := minSamples + maxBars + 8
		leadBars := 3
		followerSeries := make([]correlation.Sample, sampleCount)

		for index := range sampleCount {
			followerSeries[index] = correlation.Sample{
				At:    start.Add(time.Duration(index) * barInterval),
				Value: 100 + float64(index)*0.5,
			}
		}

		anchorSeries := make([]correlation.Sample, sampleCount)

		for index := range sampleCount {
			anchorSeries[index] = correlation.Sample{
				At:    start.Add(time.Duration(index)*barInterval + time.Duration(leadBars)*barInterval),
				Value: 200 + float64(index)*0.5,
			}
		}

		lagBars, corr, ok := CrossLagScore(anchorSeries, followerSeries, barInterval)

		Convey("It should detect leading with a negative lag", func() {
			So(ok, ShouldBeTrue)
			So(lagBars, ShouldBeLessThan, 0)
			So(corr, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkLagMeasure(b *testing.B) {
	lag := NewLag()
	input := LagInput{
		Price:       100,
		MoveReady:   true,
		MoveMoved:   true,
		LagOK:       true,
		LagBars:     3,
		LagCorr:     0.8,
		SampleCount: 16,
	}

	b.ReportAllocs()

	for b.Loop() {
		_, _ = lag.Measure(input)
	}
}
