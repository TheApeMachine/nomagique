package equation

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestStageState(testingTB *testing.T) {
	Convey("Given no inbound frame bytes", testingTB, func() {
		state, err := stageState(nil)

		Convey("It should report EOF without manufacturing state", func() {
			So(state, ShouldBeNil)
			So(err, ShouldEqual, io.EOF)
		})
	})
}
