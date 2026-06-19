package adaptive

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var varianceInput = datura.Acquire("test", datura.APPJSON).Poke(10, "sample")

func TestVarianceRead(t *testing.T) {
	Convey("Given a Variance", t, func() {
		variance := NewVariance()
		io.Copy(variance, varianceInput)

		Convey("When Read is called", func() {
			_, err := variance.Read([]byte{1, 2, 3})
			So(err, ShouldBeNil)
			So(datura.Peek[float64](variance.artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestVarianceWrite(t *testing.T) {
	Convey("Given a Variance", t, func() {
		variance := NewVariance()

		Convey("When Write is called", func() {
			_, err := io.Copy(variance, varianceInput)
			So(err, ShouldBeNil)
		})
	})
}
