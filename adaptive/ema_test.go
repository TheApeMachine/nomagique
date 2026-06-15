package adaptive

import (
	"encoding/binary"
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func TestEMA_step(testingTB *testing.T) {
	cases := []struct {
		name    string
		samples []float64
		expect  float64
	}{
		{"bootstrap echo", []float64{10}, 10},
		{"collapsed repeat", []float64{10, 10, 10}, 10},
		{"unit step up", []float64{0, 10}, 10},
		{"full retrace", []float64{10, 20, 5}, 5},
		{"negative bootstrap", []float64{-5}, -5},
		{"oscillating range", []float64{1, 3, 1, 3}, 3},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			ema := NewEMA()
			var got float64

			for _, sample := range testCase.samples {
				got = ema.step(sample)
			}

			Convey("It should derive the expected value", func() {
				So(got, ShouldEqual, testCase.expect)
			})
		})
	}
}

func TestEMA_ReadWrite(testingTB *testing.T) {
	Convey("Given Write then Read", testingTB, func() {
		ema := NewEMA()
		in := datura.Acquire("test", datura.Artifact_Type_json)
		payload := make([]byte, 8)
		binary.BigEndian.PutUint64(payload, math.Float64bits(10))
		_ = in.SetPayload(payload)
		buf, _ := in.Message().Marshal()

		_, err := ema.Write(buf)

		So(err, ShouldBeNil)

		out := make([]byte, len(buf))
		_, err = ema.Read(out)

		So(err == nil || err == io.EOF, ShouldBeTrue)

		result := datura.Acquire("out", datura.Artifact_Type_json)
		_, _ = result.Write(out)
		readPayload, _ := result.Payload()

		So(math.Float64frombits(binary.BigEndian.Uint64(readPayload)), ShouldEqual, 10)
	})
}
