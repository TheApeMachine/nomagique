package probability

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/core"
)

func TestCUSUM(testingTB *testing.T) {
	Convey("Given CUSUM constructor", testingTB, func() {
		changeSum := CUSUM()

		Convey("It should return a usable dynamic", func() {
			So(changeSum, ShouldNotBeNil)
		})
	})
}

func TestChangeSum_Observe(testingTB *testing.T) {
	Convey("Given a change sum", testingTB, func() {
		changeSum := CUSUM()
		changeSum.Observe(core.Float64(10))
		value := changeSum.Observe(core.Float64(25))

		Convey("It should accumulate evidence", func() {
			So(float64(value), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkCUSUM_Observe(testingTB *testing.B) {
	changeSum := CUSUM()
	changeSum.Observe(core.Float64(10))

	for testingTB.Loop() {
		changeSum.Observe(core.Float64(10.5))
	}
}
