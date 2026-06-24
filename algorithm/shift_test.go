package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func shiftWire(artifact *datura.Artifact, observed float64, expected float64) *datura.Artifact {
	artifact.Poke("features", "root")
	artifact.Poke([]string{"sample", "paired"}, "inputs")
	artifact.Merge("features", []float64{observed, expected})

	return artifact
}

func TestShift_Observe(testingTB *testing.T) {
	Convey("Given matching reference and live distributions", testingTB, func() {
		shift := NewShift(0, 0)
		artifact := datura.Acquire("shift-test", datura.APPJSON)

		for range 4 {
			_ = transport.NewFlipFlop(shiftWire(artifact, 1.0, 1.0), shift)
		}

		Convey("It should return zero drift", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldAlmostEqual, 0, 1e-9)
		})
	})

	Convey("Given diverging reference and live distributions", testingTB, func() {
		shift := NewShift(0, 0)
		artifact := datura.Acquire("shift-test", datura.APPJSON)
		pairs := []struct {
			observed float64
			expected float64
		}{
			{4, 1}, {1, 1}, {1, 1}, {1, 4},
		}

		for _, pair := range pairs {
			_ = transport.NewFlipFlop(shiftWire(artifact, pair.observed, pair.expected), shift)
		}

		Convey("It should return positive drift", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkShift_Observe(testingTB *testing.B) {
	shift := NewShift(0, 0)
	artifact := datura.Acquire("shift-bench", datura.APPJSON)
	pairs := []struct {
		observed float64
		expected float64
	}{
		{1, 2}, {1, 1}, {2, 4}, {1, 1},
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		for _, pair := range pairs {
			_ = transport.NewFlipFlop(shiftWire(artifact, pair.observed, pair.expected), shift)
		}
	}
}
