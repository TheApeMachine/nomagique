package geometry

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestVec512_helpers(testingTB *testing.T) {
	Convey("Given small vectors", testingTB, func() {
		left := []float64{1, 2, 3}
		right := []float64{4, 5, 6}
		dst := make([]float64, len(left))

		vecMul(dst, left, right)
		vecAdd(dst, left, right)
		vecScale(dst, left, 2)
		vecAddScalar(dst, left, 1)

		Convey("It should reduce vector primitives consistently", func() {
			So(vecSum(left), ShouldEqual, 6)
			So(vecDotProduct(left, right), ShouldEqual, 32)
			So(vecSumOfSquares(left), ShouldEqual, 14)
			So(vecMax(left), ShouldEqual, 3)
		})
	})

	Convey("Given phase vectors", testingTB, func() {
		phases := []float64{0, math.Pi / 2}
		sinDst := make([]float64, len(phases))
		cosDst := make([]float64, len(phases))

		vecSinCos(sinDst, cosDst, phases)

		Convey("It should populate sincos pairs", func() {
			So(sinDst[0], ShouldAlmostEqual, 0, 1e-12)
			So(cosDst[0], ShouldAlmostEqual, 1, 1e-12)
			So(sinDst[1], ShouldAlmostEqual, 1, 1e-12)
			So(cosDst[1], ShouldAlmostEqual, 0, 1e-12)
		})
	})
}
