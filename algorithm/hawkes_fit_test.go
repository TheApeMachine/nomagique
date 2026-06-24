package algorithm

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
)

func TestHawkesFit_Observe(testingTB *testing.T) {
	Convey("Given timestamp arrival streams with enough events", testingTB, func() {
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

		horizon := float64(start.Add(4 * time.Second).UnixNano())
		fitProcess := NewHawkesFit(hawkesFitConfig(horizon))
		inbound := datura.Acquire("hawkes-fit-test", datura.APPJSON).
			Poke(float64(len(xTimes)), "config", "xCount").
			Poke(float64(len(yTimes)), "config", "yCount").
			WithPayload(equation.MarshalFeaturesPayload(append(xTimes, yTimes...)))
		frame := inbound.Pack()

		So(len(frame), ShouldBeGreaterThan, 0)

		_, err := fitProcess.Write(frame)

		So(err, ShouldBeNil)

		outbound, err := readOutbound(fitProcess)

		So(err, ShouldBeNil)

		Convey("It should fit and return a positive excitation ratio", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkHawkesFit_Observe(testingTB *testing.B) {
	start := time.Now()
	xTimes := make([]float64, 32)
	yTimes := make([]float64, 32)

	for index := range xTimes {
		xTimes[index] = float64(start.Add(time.Duration(index) * 100 * time.Millisecond).UnixNano())
		yTimes[index] = float64(start.Add(time.Duration(index)*100*time.Millisecond + 50*time.Millisecond).UnixNano())
	}

	horizon := float64(start.Add(4 * time.Second).UnixNano())
	fitProcess := NewHawkesFit(hawkesFitConfig(horizon))
	inbound := datura.Acquire("hawkes-fit-bench", datura.APPJSON).
		Poke(float64(len(xTimes)), "config", "xCount").
		Poke(float64(len(yTimes)), "config", "yCount").
		WithPayload(equation.MarshalFeaturesPayload(append(xTimes, yTimes...)))
	frame := inbound.Pack()
	readFrame := make([]byte, 4096)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = fitProcess.Write(frame)
		_, _ = fitProcess.Read(readFrame)
	}
}
