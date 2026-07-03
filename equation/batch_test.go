package equation

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
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

func TestEmitOutput(testingTB *testing.T) {
	Convey("Given output fields from a map", testingTB, func() {
		state := datura.Acquire("equation-test", datura.APPJSON)
		payload := make([]byte, 4096)

		n, err := emitOutput(state, payload, datura.Map[float64]{
			"zeta":  3,
			"value": 2,
			"alpha": 1,
		})

		So(err == nil || err == io.EOF, ShouldBeTrue)

		outbound := datura.Acquire("equation-test-out", datura.APPJSON)
		defer outbound.Release()

		_, err = outbound.Unpack(payload[:n])

		So(err == nil || err == io.EOF, ShouldBeTrue)

		Convey("It should stamp deterministic input order", func() {
			So(datura.Peek[[]string](outbound, "inputs"), ShouldResemble, []string{
				"alpha",
				"value",
				"zeta",
				"strength",
			})
		})
	})
}
