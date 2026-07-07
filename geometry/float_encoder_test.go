package geometry

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFloatEncoderEncode(t *testing.T) {
	Convey("Given a FloatEncoder", t, func() {
		encoder := NewFloatEncoder()

		Convey("When encoding an empty feature map", func() {
			dial := encoder.Encode(nil)

			Convey("It should return a valid zero PhaseDial", func() {
				So(len(dial), ShouldEqual, PhaseDialDimensions)
			})
		})

		Convey("When encoding a single feature vector", func() {
			features := map[string]float64{
				"spectralRadius": 0.92,
				"asymmetry":      0.15,
			}

			dial := encoder.Encode(features)

			Convey("It should produce a unit-normalized PhaseDial", func() {
				So(len(dial), ShouldEqual, PhaseDialDimensions)
				So(dialMagnitude(dial), ShouldAlmostEqual, 1.0, 0.001)
			})
		})

		Convey("When encoding two vectors with different magnitudes", func() {
			mild := map[string]float64{
				"spectralRadius": 0.90,
				"asymmetry":      0.10,
			}
			critical := map[string]float64{
				"spectralRadius": 0.99,
				"asymmetry":      0.10,
			}

			dialMild := encoder.Encode(mild)
			dialCritical := encoder.Encode(critical)

			Convey("It should produce distinct dials", func() {
				selfSimilarity := dialMild.Similarity(dialMild)
				crossSimilarity := dialMild.Similarity(dialCritical)

				So(selfSimilarity, ShouldAlmostEqual, 1.0, 0.001)
				So(crossSimilarity, ShouldBeLessThan, selfSimilarity)
			})
		})

		Convey("When training normalization statistics", func() {
			for range 100 {
				encoder.Update(map[string]float64{
					"reynolds":       500 + float64(100)*0.5,
					"spectralRadius": 0.5,
				})
			}

			encoder.Update(map[string]float64{
				"reynolds":       500,
				"spectralRadius": 0.5,
			})
			encoder.Update(map[string]float64{
				"reynolds":       1500,
				"spectralRadius": 0.5,
			})

			lowReynolds := encoder.Encode(map[string]float64{
				"reynolds":       500,
				"spectralRadius": 0.5,
			})
			highReynolds := encoder.Encode(map[string]float64{
				"reynolds":       1500,
				"spectralRadius": 0.5,
			})

			Convey("It should distinguish different magnitudes after normalization", func() {
				similarity := lowReynolds.Similarity(highReynolds)
				So(similarity, ShouldBeLessThan, 0.999)
				So(encoder.DimensionCount(), ShouldEqual, 2)
			})
		})
	})
}

func TestFloatEncoderDeterminism(t *testing.T) {
	Convey("Given the same feature map encoded twice", t, func() {
		encoder := NewFloatEncoder()
		features := map[string]float64{
			"alpha": 0.5,
			"beta":  1.2,
			"gamma": -3.7,
		}

		dialFirst := encoder.Encode(features)
		dialSecond := encoder.Encode(features)

		Convey("It should produce identical dials", func() {
			similarity := dialFirst.Similarity(dialSecond)
			So(similarity, ShouldAlmostEqual, 1.0, 0.0001)
		})
	})
}

func TestFloatEncoderHeterogeneousScales(t *testing.T) {
	Convey("Given features on wildly different scales", t, func() {
		encoder := NewFloatEncoder()

		for index := range 50 {
			encoder.Update(map[string]float64{
				"reynolds":  float64(index*100) + 500,
				"asymmetry": float64(index)*0.04 - 1.0,
			})
		}

		lowState := encoder.Encode(map[string]float64{
			"reynolds":  600,
			"asymmetry": -0.8,
		})
		highState := encoder.Encode(map[string]float64{
			"reynolds":  4500,
			"asymmetry": 0.8,
		})

		Convey("It should produce dials that reflect both dimensions", func() {
			similarity := lowState.Similarity(highState)
			So(similarity, ShouldBeLessThan, 0.95)
		})
	})
}

func BenchmarkFloatEncoderEncode(benchmarkTB *testing.B) {
	encoder := NewFloatEncoder()
	features := map[string]float64{
		"spectralRadius": 0.92,
		"asymmetry":      0.15,
		"branchingRatio": 0.87,
		"reynolds":       1200,
		"vorticity":      0.45,
		"turbulence":     0.33,
	}

	for range 100 {
		encoder.Update(features)
	}

	benchmarkTB.ResetTimer()
	benchmarkTB.ReportAllocs()

	for benchmarkTB.Loop() {
		_ = encoder.Encode(features)
	}
}

func dialMagnitude(dial PhaseDial) float64 {
	var sumSq float64

	for _, component := range dial {
		re, im := real(component), imag(component)
		sumSq += re*re + im*im
	}

	return math.Sqrt(sumSq)
}
