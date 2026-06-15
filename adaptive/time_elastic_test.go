package adaptive

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewTimeElastic(testingTB *testing.T) {
	Convey("Given NewTimeElastic", testingTB, func() {
		stage := NewTimeElastic(time.Hour, 0)

		Convey("It should return a usable stage", func() {
			So(stage, ShouldNotBeNil)
		})
	})
}

func TestTimeElastic_Observe(testingTB *testing.T) {
	Convey("Given sample and timestamp scalars", testingTB, func() {
		stage := NewTimeElastic(time.Hour, 1e-6)
		start := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

		got := observeWithWork(stage, 10, float64(start.UnixNano()))

		Convey("It should seed unity on cold start", func() {
			So(got, ShouldEqual, 1.0)
		})
	})

	Convey("Given fewer than two scalars", testingTB, func() {
		stage := NewTimeElastic(time.Hour, 1e-6)

		got := observeInputs(stage, 10)

		Convey("It should return zero output", func() {
			So(got, ShouldEqual, 0.0)
		})
	})
}

func TestTimeElastic_Reset(testingTB *testing.T) {
	Convey("Given a reset stage", testingTB, func() {
		stage := NewTimeElastic(time.Hour, 1e-6)
		start := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

		_ = observeWithWork(stage, 10, float64(start.UnixNano()))

		err := stage.Reset()

		Convey("It should clear derived state", func() {
			So(err, ShouldBeNil)
			So(stage.memory.Initialized(), ShouldBeFalse)
		})
	})
}

func BenchmarkTimeElastic_Observe(b *testing.B) {
	stage := NewTimeElastic(time.Hour, 1e-6)
	at := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	b.ReportAllocs()

	for b.Loop() {
		_ = observeWithWork(stage, 10, float64(at.UnixNano()))

		at = at.Add(time.Millisecond)
	}
}
