package adaptive

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestCompression(testingTB *testing.T) {
	Convey("Given Compression constructor", testingTB, func() {
		squeeze := Compression()

		Convey("It should return a usable dynamic", func() {
			So(squeeze, ShouldNotBeNil)
		})
	})
}

func TestSqueeze_Observe(testingTB *testing.T) {
	Convey("Given a fresh compression dynamic", testingTB, func() {
		squeeze := Compression()
		value := squeeze.Observe(core.Float64(10))

		Convey("It should return zero on bootstrap", func() {
			So(value, ShouldEqual, core.Float64(0))
		})
	})
}

func TestSqueeze_Reset(testingTB *testing.T) {
	Convey("Given an observed compression dynamic", testingTB, func() {
		squeeze := Compression()
		_ = squeeze.Observe(core.Float64(10))

		Convey("When reset", func() {
			So(squeeze.Reset(), ShouldBeNil)
			value := squeeze.Observe(core.Float64(20))

			Convey("It should bootstrap again", func() {
				So(value, ShouldEqual, core.Float64(0))
			})
		})
	})
}
