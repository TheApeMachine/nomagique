package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestRange(testingTB *testing.T) {
	Convey("Given Range constructor", testingTB, func() {
		span := Range()

		Convey("It should return a usable dynamic", func() {
			So(span, ShouldNotBeNil)
		})
	})
}

func TestSpan_Observe(testingTB *testing.T) {
	Convey("Given a fresh range dynamic", testingTB, func() {
		span := Range()
		_ = span.Observe(core.Float64(10))
		value := span.Observe(core.Float64(25))

		Convey("It should track the running span", func() {
			So(value, ShouldEqual, core.Float64(15))
		})
	})
}

func TestSpan_Reset(testingTB *testing.T) {
	Convey("Given an observed range dynamic", testingTB, func() {
		span := Range()
		_ = span.Observe(core.Float64(10))

		Convey("When reset", func() {
			So(span.Reset(), ShouldBeNil)
			value := span.Observe(core.Float64(20))

			Convey("It should bootstrap again", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}
