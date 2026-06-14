package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique"
	"github.com/theapemachine/nomagique/causal"
	"github.com/theapemachine/nomagique/core"
)

func TestDeriveLadderBandwidth(testingTB *testing.T) {
	Convey("Given enough aligned node rows", testingTB, func() {
		left := nomagique.Numbers(
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
			10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
		)
		right := nomagique.Numbers(
			0, 2, 4, 6, 8, 10, 12, 14, 16, 18,
			20, 22, 24, 26, 28, 30, 32, 34, 36, 38,
		)
		bandwidth := deriveLadderBandwidth([]core.Numbers{left, right}, 0)

		Convey("It should return a positive Silverman-style bandwidth", func() {
			So(bandwidth, ShouldBeGreaterThan, 0)
		})
	})
}

func TestApplyDerivedLadderConfig(testingTB *testing.T) {
	Convey("Given zero ladder config fields", testingTB, func() {
		left := nomagique.Numbers(
			0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
			10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
		)
		right := nomagique.Numbers(
			0, 2, 4, 6, 8, 10, 12, 14, 16, 18,
			20, 22, 24, 26, 28, 30, 32, 34, 36, 38,
		)
		config := applyDerivedLadderConfig(causal.LadderConfig{
			TreatmentNormal: 0,
		}, []core.Numbers{left, right})

		Convey("It should derive bandwidth and confound fraction", func() {
			So(config.KernelBandwidth, ShouldBeGreaterThan, 0)
			So(config.ConfoundFraction, ShouldBeGreaterThan, 0)
		})
	})
}
