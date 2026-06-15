package adaptive

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestNewTimeElasticMemory(testingTB *testing.T) {
	Convey("Given NewTimeElasticMemory", testingTB, func() {
		memory := NewTimeElasticMemory(time.Hour, 0)

		Convey("It should return a usable memory", func() {
			So(memory, ShouldNotBeNil)
			So(memory.Initialized(), ShouldBeFalse)
		})
	})

	Convey("Given a non-positive epsilon", testingTB, func() {
		memory := NewTimeElasticMemory(time.Hour, 0)

		Convey("It should apply the default floor", func() {
			So(memory.epsilon, ShouldEqual, 1e-6)
		})
	})
}

func TestTimeElasticMemoryUpdate(testingTB *testing.T) {
	halflife := time.Hour
	start := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		halflife time.Duration
		epsilon  float64
		events   []struct {
			at     time.Time
			sample float64
		}
		expectErr bool
		assert    func(relative float64, memory *TimeElasticMemory)
	}{
		{
			name:     "cold start seeds unity",
			halflife: halflife,
			epsilon:  0,
			events: []struct {
				at     time.Time
				sample float64
			}{{start, 10}},
			assert: func(relative float64, memory *TimeElasticMemory) {
				So(relative, ShouldEqual, 1.0)
				So(memory.Initialized(), ShouldBeTrue)
			},
		},
		{
			name:     "burst after dense baseline",
			halflife: halflife,
			epsilon:  0,
			events: []struct {
				at     time.Time
				sample float64
			}{
				{start, 1},
				{start.Add(200 * time.Millisecond), 20},
			},
			assert: func(relative float64, memory *TimeElasticMemory) {
				So(relative, ShouldBeGreaterThan, 1.0)
			},
		},
		{
			name:     "long silence on thin pair",
			halflife: time.Minute,
			epsilon:  1e-6,
			events: []struct {
				at     time.Time
				sample float64
			}{
				{start, 1000},
				{start.Add(time.Second), 1000},
				{start.Add(10 * time.Minute), 5},
			},
			assert: func(relative float64, memory *TimeElasticMemory) {
				So(relative, ShouldBeLessThan, 0.1)
			},
		},
		{
			name:     "zero sample is valid",
			halflife: halflife,
			epsilon:  0,
			events: []struct {
				at     time.Time
				sample float64
			}{{start, 0}},
			assert: func(relative float64, memory *TimeElasticMemory) {
				So(relative, ShouldAlmostEqual, 1.0, 1e-6)
			},
		},
		{
			name:     "clock rewind clamps delta",
			halflife: halflife,
			epsilon:  0,
			events: []struct {
				at     time.Time
				sample float64
			}{
				{start, 10},
				{start.Add(time.Hour), 10},
				{start.Add(30 * time.Minute), 10},
			},
			assert: func(relative float64, memory *TimeElasticMemory) {
				So(relative, ShouldAlmostEqual, 1.0, 1e-6)
			},
		},
		{
			name:     "identical timestamp keeps alpha zero",
			halflife: halflife,
			epsilon:  0,
			events: []struct {
				at     time.Time
				sample float64
			}{
				{start, 10},
				{start, 20},
			},
			assert: func(relative float64, memory *TimeElasticMemory) {
				So(relative, ShouldAlmostEqual, 2.0, 1e-6)
			},
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			memory := NewTimeElasticMemory(testCase.halflife, testCase.epsilon)

			var (
				relative float64
				err      error
			)

			for _, event := range testCase.events {
				relative, err = memory.Update(event.at, event.sample)

				if testCase.expectErr {
					break
				}

				So(err, ShouldBeNil)
			}

			if testCase.expectErr {
				Convey("It should return an error", func() {
					So(err, ShouldNotBeNil)
				})

				return
			}

			Convey("It should satisfy the case assertion", func() {
				testCase.assert(relative, memory)
			})
		})
	}

	errorCases := []struct {
		name  string
		setup func() (*TimeElasticMemory, time.Time, float64)
	}{
		{
			name: "negative sample",
			setup: func() (*TimeElasticMemory, time.Time, float64) {
				return NewTimeElasticMemory(halflife, 0), start, -1
			},
		},
		{
			name: "zero timestamp",
			setup: func() (*TimeElasticMemory, time.Time, float64) {
				return NewTimeElasticMemory(halflife, 0), time.Time{}, 10
			},
		},
		{
			name: "non-positive halflife",
			setup: func() (*TimeElasticMemory, time.Time, float64) {
				return NewTimeElasticMemory(0, 0), start, 10
			},
		},
	}

	for _, testCase := range errorCases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			memory, at, sample := testCase.setup()

			_, err := memory.Update(at, sample)

			Convey("It should return an error", func() {
				So(err, ShouldNotBeNil)
			})
		})
	}
}

func TestNewTimeElastic(testingTB *testing.T) {
	Convey("Given NewTimeElastic", testingTB, func() {
		stage := NewTimeElastic[float64](time.Hour, 0)

		Convey("It should return a usable stage", func() {
			So(stage, ShouldNotBeNil)
		})
	})
}

func TestTimeElastic_Observe(testingTB *testing.T) {
	Convey("Given sample and timestamp scalars", testingTB, func() {
		stage := NewTimeElastic[float64](time.Hour, 1e-6)
		start := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

		got := float64(stage.Observe(
			core.Scalar[float64](10),
			core.Scalar[float64](float64(start.UnixNano())),
		))

		Convey("It should seed unity on cold start", func() {
			So(got, ShouldEqual, 1.0)
		})
	})

	Convey("Given fewer than two scalars", testingTB, func() {
		stage := NewTimeElastic[float64](time.Hour, 1e-6)

		got := float64(stage.Observe(core.Scalar[float64](10)))

		Convey("It should return zero output", func() {
			So(got, ShouldEqual, 0.0)
		})
	})
}

func TestTimeElastic_Reset(testingTB *testing.T) {
	Convey("Given a reset stage", testingTB, func() {
		stage := NewTimeElastic[float64](time.Hour, 1e-6)
		start := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)
		_ = stage.Observe(
			core.Scalar[float64](10),
			core.Scalar[float64](float64(start.UnixNano())),
		)

		err := stage.Reset()

		Convey("It should clear derived state", func() {
			So(err, ShouldBeNil)
			So(stage.memory.Initialized(), ShouldBeFalse)
		})
	})
}

func BenchmarkTimeElastic_Observe(b *testing.B) {
	stage := NewTimeElastic[float64](time.Hour, 1e-6)
	at := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	b.ReportAllocs()

	for b.Loop() {
		_ = stage.Observe(
			core.Scalar[float64](10),
			core.Scalar[float64](float64(at.UnixNano())),
		)

		at = at.Add(time.Millisecond)
	}
}

func BenchmarkTimeElasticMemoryUpdate(b *testing.B) {
	memory := NewTimeElasticMemory(time.Hour, 1e-6)
	at := time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC)

	b.ReportAllocs()

	for b.Loop() {
		_, err := memory.Update(at, 10)

		if err != nil {
			b.Fatal(err)
		}

		at = at.Add(time.Millisecond)
	}
}
