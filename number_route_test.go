package nomagique

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/core"
)

func TestRegisterObserveBinder(testingTB *testing.T) {
	Convey("Given Number with EMA then delta", testingTB, func() {
		exponential := adaptive.EMA()
		normalized := adaptive.Delta()
		boundary, err := Number(exponential, normalized)

		So(err, ShouldBeNil)

		boundApply, ok := boundObserveApply(
			core.Float64(boundary),
			[]core.Number{exponential, normalized},
		)

		So(ok, ShouldBeTrue)

		referenceEMA := adaptive.EMA()
		referenceDelta := adaptive.Delta()
		_, err = Number(referenceEMA, referenceDelta)

		So(err, ShouldBeNil)

		directApply := adaptive.BindObserveSample(
			[]core.Number{referenceEMA, referenceDelta},
		)

		Convey("It should register the same fast path as BindObserveSample", func() {
			So(boundApply(10), ShouldEqual, directApply(10))
		})
	})
}

func TestStageObserveApply(testingTB *testing.T) {
	Convey("Given registered one-stage observe binder", testingTB, func() {
		exponential := adaptive.EMA()
		_, err := Number(exponential)

		So(err, ShouldBeNil)

		apply, ok := stageObserveApply([]core.Number{exponential})

		Convey("It should expose a sample apply closure", func() {
			So(ok, ShouldBeTrue)
			So(apply(10), ShouldEqual, float64(Scalar(10).Observe(exponential)))
		})
	})
}

func TestBoundObserveApply_stageMismatch(testingTB *testing.T) {
	Convey("Given a bound boundary with different stage instances", testingTB, func() {
		firstEMA := adaptive.EMA()
		secondEMA := adaptive.EMA()
		boundary, err := Number(firstEMA)

		So(err, ShouldBeNil)

		_, ok := boundObserveApply(core.Float64(boundary), []core.Number{secondEMA})

		Convey("It should reject mismatched stage pointers", func() {
			So(ok, ShouldBeFalse)
		})
	})
}
