package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestCompression(testingTB *testing.T) {
	Convey("Given Compression constructor", testingTB, func() {
		compression := Compression()

		Convey("It should return a usable dynamic", func() {
			So(compression, ShouldNotBeNil)
		})
	})
}

func TestCompression_Observe(testingTB *testing.T) {
	Convey("Given a fresh compression scorer", testingTB, func() {
		compression := Compression()

		Convey("When bootstrapping", func() {
			value := compression.Observe(core.Float64(10))

			Convey("It should return zero without error", func() {
				So(value, ShouldEqual, 0)
			})
		})
	})

	Convey("Given compression history", testingTB, func() {
		compression := Compression()
		compression.Observe(core.Float64(100))

		Convey("When spread tightens", func() {
			value := compression.Observe(core.Float64(75))

			Convey("It should return a positive score", func() {
				So(value, ShouldEqual, 0.25)
			})
		})
	})

	Convey("Given pipeline work samples", testingTB, func() {
		compressor := Compression()
		compressor.Observe(core.Float64(100))
		value := compressor.Observe(core.Float64(0), core.Float64(50))

		Convey("It should score using the work sample", func() {
			So(value, ShouldEqual, 0.5)
		})
	})

	Convey("Given invalid stage inputs", testingTB, func() {
		compression := Compression()

		Convey("When observing", func() {
			value := compression.Observe(blankNumber{})

			Convey("It should return zero", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}

func TestCompression_ObserveSample(testingTB *testing.T) {
	Convey("Given compression", testingTB, func() {
		compression := Compression()
		_ = compression.ObserveSample(100)

		Convey("When spread tightens", func() {
			value := compression.ObserveSample(50)

			Convey("It should return the compression score", func() {
				So(value, ShouldEqual, 0.5)
			})
		})
	})
}

func TestCompression_ObserveSamples(testingTB *testing.T) {
	Convey("Given compression", testingTB, func() {
		compression := Compression()
		samples := []float64{100, 50}
		out := make([]float64, len(samples))

		Convey("When observing samples in batch", func() {
			compression.ObserveSamples(samples, out)

			Convey("It should fill the output buffer", func() {
				So(out[1], ShouldEqual, 0.5)
			})
		})
	})
}

func TestCompression_Reset(testingTB *testing.T) {
	Convey("Given compression with state", testingTB, func() {
		compression := Compression()
		compression.Observe(core.Float64(3))

		Convey("When reset", func() {
			err := compression.Reset()
So(err, ShouldBeNil)

			Convey("It should clear derived state", func() {
				So(compression.state.Ready, ShouldBeFalse)
			})
		})
	})
}

func BenchmarkCompression_Observe(testingTB *testing.B) {
	compression := Compression()
	compression.Observe(core.Float64(10))

	for testingTB.Loop() {
		compression.Observe(core.Float64(9))
	}
}

func BenchmarkCompression_ObserveSamples(testingTB *testing.B) {
	compression := Compression()
	samples := make([]float64, 1024)
	out := make([]float64, len(samples))

	for testingTB.Loop() {
		compression.state.Reset()
		compression.ObserveSamples(samples, out)
	}
}
