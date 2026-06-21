package hawkes

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func encodeFitPayload(samples ...float64) []byte {
	payload := make([]byte, 8*len(samples))

	for index, sample := range samples {
		offset := index * 8
		binary.BigEndian.PutUint64(payload[offset:offset+8], math.Float64bits(sample))
	}

	return payload
}

func TestFitTimesFromBinaryPayload(testingTB *testing.T) {
	Convey("Given binary arrival timestamps on the artifact payload", testingTB, func() {
		start := time.Now()
		xTimes := make([]float64, 32)
		yTimes := make([]float64, 32)

		for index := range xTimes {
			xTimes[index] = float64(
				start.Add(time.Duration(index) * 100 * time.Millisecond).UnixNano(),
			)
			yTimes[index] = float64(
				start.Add(time.Duration(index)*100*time.Millisecond + 50*time.Millisecond).UnixNano(),
			)
		}

		inbound := datura.Acquire("hawkes-fit-stage-test", datura.APPJSON).
			Poke(float64(len(xTimes)), "config", "xCount").
			Poke(float64(len(yTimes)), "config", "yCount").
			WithPayload(encodeFitPayload(append(xTimes, yTimes...)...))

		frame, frameErr := inbound.MarshalPacked()

		So(frameErr, ShouldBeNil)

		fitStage := NewFit(start.Add(4*time.Second).UnixNano(), BivariateFit{})
		_, writeErr := fitStage.Write(frame)

		So(writeErr, ShouldBeNil)

		xDecoded, yDecoded, ok := fitTimes(fitStage.artifact)

		Convey("It should decode both arrival streams", func() {
			So(ok, ShouldBeTrue)
			So(len(xDecoded), ShouldEqual, len(xTimes))
			So(len(yDecoded), ShouldEqual, len(yTimes))
		})
	})
}
