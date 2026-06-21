package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
	"github.com/theapemachine/nomagique/tests"
)

func decayPayload(
	lastPrice float64,
	bidDepths, askDepths, densities, spreads, pressures, imbalances []float64,
) []float64 {
	payload := []float64{lastPrice}

	series := [][]float64{
		bidDepths,
		askDepths,
		densities,
		spreads,
		pressures,
		imbalances,
	}

	for _, segment := range series {
		payload = append(payload, float64(len(segment)))
	}

	for _, segment := range series {
		payload = append(payload, segment...)
	}

	return payload
}

func TestDecayEvaluate(testingTB *testing.T) {
	Convey("Given deteriorating long-side book history", testingTB, func() {
		decay := equation.NewDecay()
		bidDepths := []float64{20, 18, 16, 14, 12, 10, 8, 6}
		askDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		densities := []float64{8, 8, 8, 8, 8, 8, 8, 8}
		spreads := []float64{4, 4, 4, 4, 4, 4, 4, 4}
		pressures := []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2}
		imbalances := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}

		writeErr := tests.WriteSamples(decay, decayPayload(
			100, bidDepths, askDepths, densities, spreads, pressures, imbalances,
		)...)
		So(writeErr, ShouldBeNil)

		frame := make([]byte, 4096)
		_, _ = decay.Read(frame)
		outbound := datura.Acquire("test-out", datura.APPJSON)
		_, _ = outbound.Write(frame)

		Convey("It should publish an eligible exhaustion outcome", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](outbound, "output", "urgency"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
		})
	})

	Convey("Given pressure fade on the long side", testingTB, func() {
		decay := equation.NewDecay()
		pressures := []float64{0.9, 0.85, 0.8, 0.75, 0.7, 0.2, 0.1, -0.1}
		bidDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		askDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		densities := []float64{8, 8, 8, 8, 8, 8, 8, 8}
		spreads := []float64{4, 4, 4, 4, 4, 4, 4, 4}
		imbalances := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}

		writeErr := tests.WriteSamples(decay, decayPayload(
			100, bidDepths, askDepths, densities, spreads, pressures, imbalances,
		)...)
		So(writeErr, ShouldBeNil)

		frame := make([]byte, 4096)
		_, _ = decay.Read(frame)
		outbound := datura.Acquire("test-out", datura.APPJSON)
		_, _ = outbound.Write(frame)

		Convey("It should classify thermal exhaustion", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
		})
	})

	Convey("Given ask-side thinning stronger than bid-side", testingTB, func() {
		decay := equation.NewDecay()
		bidDepths := []float64{10, 10, 10, 10, 9, 9, 9, 9}
		askDepths := []float64{10, 10, 10, 10, 8, 6, 4, 2}
		densities := []float64{8, 8, 8, 8, 8, 8, 8, 8}
		spreads := []float64{4, 4, 4, 4, 4, 4, 4, 4}
		pressures := []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2}
		imbalances := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}

		writeErr := tests.WriteSamples(decay, decayPayload(
			100, bidDepths, askDepths, densities, spreads, pressures, imbalances,
		)...)
		So(writeErr, ShouldBeNil)

		frame := make([]byte, 4096)
		_, _ = decay.Read(frame)
		outbound := datura.Acquire("test-out", datura.APPJSON)
		_, _ = outbound.Write(frame)

		Convey("It should let the stronger short-side score win", func() {
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
		})
	})
}

func BenchmarkDecayRead(b *testing.B) {
	decay := equation.NewDecay()
	samples := decayPayload(
		100,
		[]float64{20, 18, 16, 14, 12, 10, 8, 6},
		[]float64{10, 10, 10, 10, 10, 10, 10, 10},
		[]float64{8, 8, 8, 8, 8, 8, 8, 8},
		[]float64{4, 4, 4, 4, 4, 4, 4, 4},
		[]float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
		[]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
	)
	frame := make([]byte, 4096)

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(decay, samples...)
		_, _ = decay.Read(frame)
	}
}
