package algorithm

import (
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/utils"
)

func TestPayloadSamples(testingTB *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		wantLen int
		wantNil bool
	}{
		{name: "empty", payload: nil, wantNil: true},
		{name: "odd length", payload: []byte{0, 0, 0, 0, 0, 0, 0}, wantNil: true},
		{name: "valid triple", payload: encodePayload(1, 2, 3), wantLen: 3},
		{name: "nan rejected", payload: encodePayload(math.NaN()), wantNil: true},
		{name: "inf rejected", payload: encodePayload(math.Inf(1)), wantNil: true},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given payloadSamples with "+testCase.name, testingTB, func() {
			samples := payloadSamples(testCase.payload)

			if testCase.wantNil {
				Convey("It should return nil", func() {
					So(samples, ShouldBeNil)
				})

				return
			}

			Convey("It should decode finite samples", func() {
				So(len(samples), ShouldEqual, testCase.wantLen)
				So(samples[0], ShouldEqual, 1)
			})
		})
	}
}

func TestEncodePayload(testingTB *testing.T) {
	Convey("Given finite samples", testingTB, func() {
		payload := encodePayload(3.5, -2.25)

		Convey("It should round-trip through payloadScalar", func() {
			first, firstOk := payloadScalar(payload[:8])
			second, secondOk := payloadScalar(payload[8:])

			So(firstOk, ShouldBeTrue)
			So(secondOk, ShouldBeTrue)
			So(first, ShouldEqual, 3.5)
			So(second, ShouldEqual, -2.25)
		})
	})
}

func TestPayloadScalar(testingTB *testing.T) {
	cases := []struct {
		name    string
		payload []byte
		wantOk  bool
	}{
		{name: "wrong length", payload: encodePayload(1, 2), wantOk: false},
		{name: "nan", payload: encodePayload(math.NaN()), wantOk: false},
		{name: "valid", payload: encodePayload(42), wantOk: true},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given payloadScalar with "+testCase.name, testingTB, func() {
			value, ok := payloadScalar(testCase.payload)

			Convey("It should report validity", func() {
				So(ok, ShouldEqual, testCase.wantOk)

				if testCase.wantOk {
					So(value, ShouldEqual, 42)
				}
			})
		})
	}
}

func TestZipNodeRows(testingTB *testing.T) {
	cases := []struct {
		name    string
		streams [][]float64
		wantOk  bool
		wantLen int
	}{
		{name: "empty", streams: nil, wantOk: false},
		{name: "empty row", streams: [][]float64{{}}, wantOk: false},
		{name: "mismatched", streams: [][]float64{{1, 2}, {1}}, wantOk: false},
		{name: "valid", streams: [][]float64{{1, 2}, {3, 4}}, wantOk: true, wantLen: 2},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given zipNodeRows with "+testCase.name, testingTB, func() {
			rows, ok := zipNodeRows(testCase.streams)

			Convey("It should zip aligned streams", func() {
				So(ok, ShouldEqual, testCase.wantOk)

				if !testCase.wantOk {
					return
				}

				So(len(rows), ShouldEqual, testCase.wantLen)
				So(rows[0], ShouldResemble, []float64{1, 3})
				So(rows[1], ShouldResemble, []float64{2, 4})
			})
		})
	}
}

func TestSamplesFromTimeValues(testingTB *testing.T) {
	cases := []struct {
		name    string
		values  []float64
		wantOk  bool
		wantLen int
	}{
		{name: "too short", values: []float64{1, 2, 3}, wantOk: false},
		{name: "odd length", values: []float64{1, 2, 3, 4, 5}, wantOk: false},
		{name: "nan seconds", values: []float64{math.NaN(), 1, 2, 3}, wantOk: false},
		{name: "inf value", values: []float64{1, math.Inf(1), 2, 3}, wantOk: false},
		{name: "valid pair", values: []float64{100, 1.5, 101, 2.5}, wantOk: true, wantLen: 2},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given samplesFromTimeValues with "+testCase.name, testingTB, func() {
			samples, ok := samplesFromTimeValues(testCase.values)

			Convey("It should parse time-value pairs", func() {
				So(ok, ShouldEqual, testCase.wantOk)

				if !testCase.wantOk {
					return
				}

				So(len(samples), ShouldEqual, testCase.wantLen)
				So(samples[0].At, ShouldEqual, time.Unix(100, 0))
				So(samples[0].Value, ShouldEqual, 1.5)
			})
		})
	}
}

func TestAppendRingFloat(testingTB *testing.T) {
	cases := []struct {
		name     string
		values   []float64
		value    float64
		capacity int
		want     []float64
	}{
		{name: "under capacity", values: []float64{1}, value: 2, capacity: 3, want: []float64{1, 2}},
		{name: "at capacity", values: []float64{1, 2}, value: 3, capacity: 2, want: []float64{2, 3}},
		{name: "over capacity", values: []float64{1, 2, 3}, value: 4, capacity: 2, want: []float64{3, 4}},
		{name: "nan appended", values: []float64{1}, value: math.NaN(), capacity: 2, want: []float64{1, math.NaN()}},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given AppendRingFloat "+testCase.name, testingTB, func() {
			result := utils.AppendRingFloat(testCase.values, testCase.value, testCase.capacity)

			Convey("It should retain trailing window", func() {
				So(len(result), ShouldEqual, len(testCase.want))

				for index, expected := range testCase.want {
					if math.IsNaN(expected) {
						So(math.IsNaN(result[index]), ShouldBeTrue)
						continue
					}

					So(result[index], ShouldEqual, expected)
				}
			})
		})
	}
}
