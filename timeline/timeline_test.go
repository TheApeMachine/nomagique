package timeline

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

var testEpoch = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

func TestNew(testingTB *testing.T) {
	cases := []struct {
		name    string
		times   []time.Time
		expect  []time.Time
		aliases bool
	}{
		{
			name:   "empty input",
			times:  nil,
			expect: nil,
		},
		{
			name:   "single event",
			times:  []time.Time{testEpoch},
			expect: []time.Time{testEpoch},
		},
		{
			name: "already sorted pair",
			times: []time.Time{
				testEpoch,
				testEpoch.Add(time.Second),
			},
			expect: []time.Time{
				testEpoch,
				testEpoch.Add(time.Second),
			},
			aliases: true,
		},
		{
			name: "unsorted triple",
			times: []time.Time{
				testEpoch.Add(3 * time.Second),
				testEpoch,
				testEpoch.Add(time.Second),
			},
			expect: []time.Time{
				testEpoch,
				testEpoch.Add(time.Second),
				testEpoch.Add(3 * time.Second),
			},
		},
		{
			name: "duplicate timestamps",
			times: []time.Time{
				testEpoch,
				testEpoch,
				testEpoch.Add(2 * time.Second),
			},
			expect: []time.Time{
				testEpoch,
				testEpoch,
				testEpoch.Add(2 * time.Second),
			},
			aliases: true,
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			input := testCase.times
			eventTimeline := New(input)

			Convey("It should expose the expected ordered times", func() {
				got := eventTimeline.Times()
				So(len(got), ShouldEqual, len(testCase.expect))

				for index, expect := range testCase.expect {
					So(got[index].Equal(expect), ShouldBeTrue)
				}
			})

			if testCase.aliases {
				Convey("It should reuse sorted input storage", func() {
					So(eventTimeline.Times(), ShouldEqual, input)
				})
			}

			if !testCase.aliases && len(testCase.times) > 1 {
				Convey("It should not mutate the caller slice when sorting", func() {
					So(input[0].Equal(testCase.times[0]), ShouldBeTrue)
				})
			}
		})
	}
}

func TestTimeline_Times(testingTB *testing.T) {
	Convey("Given a constructed timeline", testingTB, func() {
		eventTimeline := New([]time.Time{testEpoch, testEpoch.Add(time.Second)})

		Convey("It should return the stored sequence", func() {
			So(len(eventTimeline.Times()), ShouldEqual, 2)
			So(eventTimeline.Times()[0].Equal(testEpoch), ShouldBeTrue)
		})
	})
}

func TestTimeline_Len(testingTB *testing.T) {
	cases := []struct {
		name   string
		times  []time.Time
		expect int
	}{
		{"empty", nil, 0},
		{"single", []time.Time{testEpoch}, 1},
		{"triple", []time.Time{testEpoch, testEpoch.Add(time.Second), testEpoch.Add(2 * time.Second)}, 3},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			eventTimeline := New(testCase.times)

			Convey("It should report the event count", func() {
				So(eventTimeline.Len(), ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestTimeline_Gaps(testingTB *testing.T) {
	cases := []struct {
		name   string
		times  []time.Time
		expect []float64
	}{
		{
			name:   "empty timeline",
			times:  nil,
			expect: nil,
		},
		{
			name:   "single event",
			times:  []time.Time{testEpoch},
			expect: nil,
		},
		{
			name: "duplicate and positive gaps",
			times: []time.Time{
				testEpoch,
				testEpoch.Add(2 * time.Second),
				testEpoch.Add(2 * time.Second),
				testEpoch.Add(5 * time.Second),
			},
			expect: []float64{2, 3},
		},
		{
			name: "all duplicates",
			times: []time.Time{
				testEpoch,
				testEpoch,
				testEpoch,
			},
			expect: nil,
		},
		{
			name: "sub-second precision",
			times: []time.Time{
				testEpoch,
				testEpoch.Add(500 * time.Millisecond),
				testEpoch.Add(1500 * time.Millisecond),
			},
			expect: []float64{0.5, 1},
		},
		{
			name: "large span",
			times: []time.Time{
				testEpoch,
				testEpoch.Add(time.Hour),
			},
			expect: []float64{3600},
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			eventTimeline := New(testCase.times)
			gaps := eventTimeline.Gaps()

			Convey("It should return strictly positive inter-arrival gaps", func() {
				if testCase.expect == nil {
					So(len(gaps), ShouldEqual, 0)

					return
				}

				So(len(gaps), ShouldEqual, len(testCase.expect))

				for index, expect := range testCase.expect {
					So(gaps[index], ShouldEqual, expect)
				}
			})
		})
	}

	Convey("Given an unsorted raw timeline", testingTB, func() {
		eventTimeline := Timeline{times: []time.Time{
			testEpoch.Add(5 * time.Second),
			testEpoch,
		}}

		Convey("It should omit non-positive gaps", func() {
			So(len(eventTimeline.Gaps()), ShouldEqual, 0)
		})
	})
}

func TestTimeline_Span(testingTB *testing.T) {
	cases := []struct {
		name    string
		times   []time.Time
		horizon time.Time
		expect  float64
	}{
		{
			name:    "empty timeline",
			times:   nil,
			horizon: testEpoch.Add(time.Hour),
			expect:  0,
		},
		{
			name:    "horizon before first event",
			times:   []time.Time{testEpoch.Add(time.Hour)},
			horizon: testEpoch,
			expect:  0,
		},
		{
			name:    "horizon equal to first event",
			times:   []time.Time{testEpoch, testEpoch.Add(time.Second)},
			horizon: testEpoch,
			expect:  0,
		},
		{
			name:    "horizon after first event",
			times:   []time.Time{testEpoch, testEpoch.Add(time.Second)},
			horizon: testEpoch.Add(4 * time.Second),
			expect:  4,
		},
		{
			name:    "single event span",
			times:   []time.Time{testEpoch},
			horizon: testEpoch.Add(90 * time.Second),
			expect:  90,
		},
		{
			name:    "sub-second span",
			times:   []time.Time{testEpoch},
			horizon: testEpoch.Add(250 * time.Millisecond),
			expect:  0.25,
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			eventTimeline := New(testCase.times)

			Convey("It should return elapsed seconds from the first event", func() {
				So(eventTimeline.Span(testCase.horizon), ShouldEqual, testCase.expect)
			})
		})
	}
}

func BenchmarkNew_Sorted(b *testing.B) {
	times := make([]time.Time, 128)

	for index := range times {
		times[index] = testEpoch.Add(time.Duration(index) * time.Millisecond)
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = New(times)
	}
}

func BenchmarkNew_Unsorted(b *testing.B) {
	times := make([]time.Time, 128)

	for index := range times {
		times[index] = testEpoch.Add(time.Duration(127-index) * time.Millisecond)
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = New(times)
	}
}

func BenchmarkTimeline_Gaps(b *testing.B) {
	times := make([]time.Time, 128)

	for index := range times {
		times[index] = testEpoch.Add(time.Duration(index) * time.Millisecond)
	}

	eventTimeline := New(times)

	b.ReportAllocs()

	for b.Loop() {
		_ = eventTimeline.Gaps()
	}
}

func BenchmarkTimeline_Span(b *testing.B) {
	times := make([]time.Time, 128)

	for index := range times {
		times[index] = testEpoch.Add(time.Duration(index) * time.Millisecond)
	}

	eventTimeline := New(times)
	horizon := testEpoch.Add(time.Hour)

	b.ReportAllocs()

	for b.Loop() {
		_ = eventTimeline.Span(horizon)
	}
}
