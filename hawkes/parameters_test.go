package hawkes

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFitFromLogParams(t *testing.T) {
	Convey("Given in-range log parameters", t, func() {
		context := FitContext{
			BranchFloor:   0.01,
			BranchCeiling: 0.9,
		}
		fit := fitFromLogParams([bivariateParamCount]float64{
			math.Log(1),
			math.Log(1),
			math.Log(1),
			math.Log(0.2),
			math.Log(0.05),
			math.Log(0.05),
			math.Log(0.2),
		}, context)

		Convey("It should produce a valid fit", func() {
			So(fit.MuX, ShouldBeGreaterThan, 0)
			So(fit.Beta, ShouldBeGreaterThan, 0)
			So(fit.SpectralRadius, ShouldBeGreaterThan, 0)
			So(fit.SpectralRadius, ShouldBeLessThan, criticalBranch)
		})
	})

	Convey("Given weak excitation below the branch floor", t, func() {
		context := FitContext{
			BranchFloor:   0.1,
			BranchCeiling: 0.9,
		}
		fit := fitFromLogParams([bivariateParamCount]float64{
			math.Log(1),
			math.Log(1),
			math.Log(1),
			math.Log(0.01),
			math.Log(0.0),
			math.Log(0.0),
			math.Log(0.01),
		}, context)

		Convey("It should retain the valid subcritical fit", func() {
			So(fit.MuX, ShouldBeGreaterThan, 0)
			So(fit.SpectralRadius, ShouldBeGreaterThan, 0)
			So(fit.SpectralRadius, ShouldBeLessThan, context.BranchFloor)
		})
	})
}
