package hawkes

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestArrivalWorkspace_Stream(testingTB *testing.T) {
	Convey("Given reusable storage and sorted side timelines", testingTB, func() {
		start := time.Unix(3000, 0)
		workspace := NewArrivalWorkspace()
		stream := workspace.Stream(
			[]time.Time{start, start.Add(2 * time.Second)},
			[]time.Time{start.Add(time.Second)},
		)

		Convey("It should expose the same merged stream and gap statistics", func() {
			So(stream.Marked(), ShouldHaveLength, 3)
			So(stream.Gaps(), ShouldResemble, []float64{1, 1})
		})
	})
}

func BenchmarkArrivalWorkspace_Stream(testingTB *testing.B) {
	const eventCount = 128

	start := time.Unix(3000, 0)
	buyTimes := make([]time.Time, 0, eventCount/2)
	sellTimes := make([]time.Time, 0, eventCount/2)

	for index := range eventCount {
		arrival := start.Add(time.Duration(index) * time.Millisecond)

		if index%2 == 0 {
			buyTimes = append(buyTimes, arrival)
			continue
		}

		sellTimes = append(sellTimes, arrival)
	}

	workspace := NewArrivalWorkspace()
	_ = workspace.Stream(buyTimes, sellTimes)
	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = workspace.Stream(buyTimes, sellTimes)
	}
}
