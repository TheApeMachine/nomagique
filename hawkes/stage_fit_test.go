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

		frame := inbound.Pack()

		So(len(frame), ShouldBeGreaterThan, 0)

		fitStage := NewFit(fitConfigArtifact(
			float64(start.Add(4*time.Second).UnixNano()),
			BivariateFit{},
		))
		_, err := fitStage.Write(frame)

		So(err, ShouldBeNil)

		state := datura.Acquire("hawkes-fit-decode-test", datura.APPJSON)
		_, writeStateErr := state.Write(fitStage.artifact.DecryptPayload())

		So(writeStateErr, ShouldBeNil)

		xDecoded, yDecoded, ok := fitTimes(state, fitStage.artifact)

		Convey("It should decode both arrival streams", func() {
			So(ok, ShouldBeTrue)
			So(len(xDecoded), ShouldEqual, len(xTimes))
			So(len(yDecoded), ShouldEqual, len(yTimes))
		})
	})
}

func TestMomentReadRequiresAlignedSamples(testingTB *testing.T) {
	Convey("Given insufficient moment samples", testingTB, func() {
		moment := NewMoment(momentConfigArtifact(
			BivariateParams{MuX: 1, MuY: 1, Beta: 1},
			1,
			1,
		))
		payload := make([]byte, 0)
		_, err := moment.Write(payload)

		So(err, ShouldBeNil)

		response := make([]byte, 4096)
		_, err = moment.Read(response)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestFitReadRequiresAlignedTimestamps(testingTB *testing.T) {
	Convey("Given insufficient fit timestamps", testingTB, func() {
		fitStage := NewFit(fitConfigArtifact(float64(time.Now().UnixNano()), BivariateFit{}))
		payload := make([]byte, 0)
		_, err := fitStage.Write(payload)

		So(err, ShouldBeNil)

		response := make([]byte, 4096)
		_, err = fitStage.Read(response)

		Convey("It should return a validation error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}
