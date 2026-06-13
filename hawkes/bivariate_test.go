package hawkes

import "testing"

func TestSpectralRadius(testingTB *testing.T) {
	matrix := [2][2]float64{
		{0.4, 0.1},
		{0.2, 0.3},
	}
	radius := SpectralRadius(matrix)

	if radius <= 0 || radius >= 1 {
		testingTB.Fatalf("expected subcritical radius, got %v", radius)
	}
}
