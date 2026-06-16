package learning

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestObserveSampleRatio(testingTB *testing.T) {
	Convey("Given ObserveSampleRatio", testingTB, func() {
		byFunction := SampleRatioState{}
		byMethod := SampleRatioState{}

		Convey("It should match method observation", func() {
			So(
				ObserveSampleRatio(&byFunction, 10, 10),
				ShouldEqual,
				byMethod.Observe(10, 10),
			)
		})
	})
}
