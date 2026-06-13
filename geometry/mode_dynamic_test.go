package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
)

func TestModeDetector_Observe(testingTB *testing.T) {
	Convey("Given coupled participants", testingTB, func() {
		origins := nomagique.Numbers(10, 20, 30)
		energies := nomagique.Numbers(1, 2, 4)
		coupling := nomagique.Numbers(
			1, 1, 0,
			1, 1, 0,
			0, 0, 1,
		)

		detector := NewModeDetector(1, origins, energies, coupling)
		energy := detector.Observe()

		Convey("It should return dominant mode energy", func() {
			So(float64(energy), ShouldEqual, 4)
			So(detector.ModeCount(), ShouldEqual, 2)
			So(float64(detector.DominantEnergy()), ShouldEqual, 4)
		})
	})
}

func BenchmarkModeDetector_Observe(testingTB *testing.B) {
	size := 16
	origins := make([]float64, size)
	energies := make([]float64, size)
	matrix := make([]float64, size*size)

	for index := range origins {
		origins[index] = float64(index + 1)
		energies[index] = float64(index%5 + 1)

		for col := range size {
			value := 0.0

			if (index+col)%3 == 0 {
				value = 1
			}

			matrix[index*size+col] = value
		}
	}

	detector := NewModeDetector(0.9, nomagique.Numbers(origins...), nomagique.Numbers(energies...), nomagique.Numbers(matrix...))

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = detector.Observe()
	}
}
