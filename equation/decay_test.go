package equation_test

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/equation"
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

func TestDecay_Read(testingTB *testing.T) {
	Convey("Given deteriorating long-side book history", testingTB, func() {
		stage := equation.NewDecay(equation.DecayConfig())
		bidDepths := []float64{20, 18, 16, 14, 12, 10, 8, 6}
		askDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		densities := []float64{8, 8, 8, 8, 8, 8, 8, 8}
		spreads := []float64{4, 4, 4, 4, 4, 4, 4, 4}
		pressures := []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2}
		imbalances := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}

		err := writeFeatureStage(stage, equation.DecayInputKeys, decayPayload(
			100, bidDepths, askDepths, densities, spreads, pressures, imbalances,
		)...)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify mechanical exhaustion", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 1)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](outbound, "output", "urgency"), ShouldEqual,
				datura.Peek[float64](outbound, "output", "value"))
		})
	})

	Convey("Given pressure fade on the long side", testingTB, func() {
		stage := equation.NewDecay(equation.DecayConfig())
		pressures := []float64{0.9, 0.85, 0.8, 0.75, 0.7, 0.2, 0.1, -0.1}
		bidDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		askDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		densities := []float64{8, 8, 8, 8, 8, 8, 8, 8}
		spreads := []float64{4, 4, 4, 4, 4, 4, 4, 4}
		imbalances := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}

		err := writeFeatureStage(stage, equation.DecayInputKeys, decayPayload(
			100, bidDepths, askDepths, densities, spreads, pressures, imbalances,
		)...)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify thermal exhaustion", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 3)
		})
	})

	Convey("Given widening spread without depth collapse", testingTB, func() {
		stage := equation.NewDecay(equation.DecayConfig())
		bidDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		askDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		densities := []float64{20, 20, 20, 20, 20, 20, 20, 20}
		spreads := []float64{1, 1, 1, 1, 1, 1, 1, 3}
		pressures := []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2}
		imbalances := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}

		err := writeFeatureStage(stage, equation.DecayInputKeys, decayPayload(
			100, bidDepths, askDepths, densities, spreads, pressures, imbalances,
		)...)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify fragile expansion", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 2)
			So(datura.Peek[float64](outbound, "output", "fragile"), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given support-side imbalance flipping against the position", testingTB, func() {
		stage := equation.NewDecay(equation.DecayConfig())
		bidDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		askDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		densities := []float64{20, 20, 20, 20, 20, 20, 20, 20}
		spreads := []float64{1, 1, 1, 1, 1, 1, 1, 1}
		pressures := []float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2}
		imbalances := []float64{0.4, 0.35, 0.3, 0.32, 0.28, 0.3, 0.25, -0.5}

		err := writeFeatureStage(stage, equation.DecayInputKeys, decayPayload(
			100, bidDepths, askDepths, densities, spreads, pressures, imbalances,
		)...)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should classify active reversal", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 4)
			So(datura.Peek[float64](outbound, "output", "reversal"), ShouldBeGreaterThan, 0)
		})
	})

	Convey("Given stable book history without decay signals", testingTB, func() {
		stage := equation.NewDecay(equation.DecayConfig())
		bidDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		askDepths := []float64{10, 10, 10, 10, 10, 10, 10, 10}
		densities := []float64{20, 20, 20, 20, 20, 20, 20, 20}
		spreads := []float64{4, 4, 4, 4, 4, 4, 4, 4}
		pressures := []float64{0, 0, 0, 0, 0, 0, 0, 0}
		imbalances := []float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1}

		err := writeFeatureStage(stage, equation.DecayInputKeys, decayPayload(
			100, bidDepths, askDepths, densities, spreads, pressures, imbalances,
		)...)

		So(err, ShouldBeNil)

		outbound, err := readStageOutput(stage)

		So(err, ShouldBeNil)

		Convey("It should emit zero urgency without rejecting the stage", func() {
			So(int(datura.Peek[float64](outbound, "output", "category")), ShouldEqual, 0)
			So(datura.Peek[float64](outbound, "output", "urgency"), ShouldEqual, 0)
			So(datura.Peek[float64](outbound, "output", "mechanical"), ShouldEqual, 0)
			So(datura.Peek[float64](outbound, "output", "fragile"), ShouldEqual, 0)
			So(datura.Peek[float64](outbound, "output", "thermal"), ShouldEqual, 0)
			So(datura.Peek[float64](outbound, "output", "reversal"), ShouldEqual, 0)
		})
	})
}

func BenchmarkDecayRead(b *testing.B) {
	stage := equation.NewDecay(equation.DecayConfig())
	values := decayPayload(
		100,
		[]float64{20, 18, 16, 14, 12, 10, 8, 6},
		[]float64{10, 10, 10, 10, 10, 10, 10, 10},
		[]float64{8, 8, 8, 8, 8, 8, 8, 8},
		[]float64{4, 4, 4, 4, 4, 4, 4, 4},
		[]float64{0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2, 0.2},
		[]float64{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
	)

	b.ReportAllocs()

	for b.Loop() {
		_ = writeFeatureStage(stage, equation.DecayInputKeys, values...)
		frame := make([]byte, 4096)
		_, _ = stage.Read(frame)
	}
}
