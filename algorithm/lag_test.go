package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/correlation"
	"github.com/theapemachine/nomagique/tests"
)

func TestLagEvaluateFollowerSync(testingTB *testing.T) {
	Convey("Given aligned follower correlation before anchor move gate warms", testingTB, func() {
		lag := NewLag(datura.Acquire("lag-config", datura.APPJSON))
		err := tests.WriteSamples(lag,
			0, 100,
			0, 0, 0,
			0, 0, 0,
			1, 0.9,
			16,
		)
		So(err, ShouldBeNil)
		_, _ = lag.Read(make([]byte, 4096))

		Convey("It should classify synchronized drift", func() {
			So(lag.outcome.Eligible, ShouldBeTrue)
			So(lag.outcome.Category, ShouldEqual, 2)
		})
	})
}

func TestLagEvaluateAnchorStall(testingTB *testing.T) {
	Convey("Given a warmed flat anchor path", testingTB, func() {
		lag := NewLag(datura.Acquire("lag-config", datura.APPJSON))
		err := tests.WriteSamples(lag,
			1, 50000,
			1, 0, 0.6,
			0, 0, 0,
			0, 0,
			0,
		)
		So(err, ShouldBeNil)
		_, _ = lag.Read(make([]byte, 4096))

		Convey("It should classify anchor stall", func() {
			So(lag.outcome.Category, ShouldEqual, 4)
			So(lag.outcome.Strength, ShouldBeGreaterThan, 0)
		})
	})
}

func TestLagReadInsufficientFields(testingTB *testing.T) {
	Convey("Given fewer than eleven lag feature fields", testingTB, func() {
		lag := NewLag(datura.Acquire("lag-config", datura.APPJSON))
		err := tests.WriteSamples(lag,
			0, 100,
			0, 0, 0,
		)
		So(err, ShouldBeNil)

		_, err = lag.Read(make([]byte, 4096))

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

func BenchmarkLagRead(b *testing.B) {
	lag := NewLag(datura.Acquire("lag-config-bench", datura.APPJSON))
	samples := []float64{
		0, 100,
		1, 1, 0,
		1, 3, 0.8,
		0, 0,
		16,
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(lag, samples...)
		_, _ = lag.Read(make([]byte, 4096))
	}
}
