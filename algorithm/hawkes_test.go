package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique/hawkes"
	"github.com/theapemachine/nomagique/tests"
)

func TestHawkes_Observe(testingTB *testing.T) {
	Convey("Given parameters fit to the configured streams", testingTB, func() {
		xStream := []float64{2, 4, 6, 8}
		yStream := []float64{1, 2, 3, 4}

		seed, ok := hawkes.MethodOfMoments(xStream, yStream, nil, 1)

		So(ok, ShouldBeTrue)

		config := hawkesMomentConfig(seed, 1, 1)
		process := NewHawkes(config)
		batch := hawkes.EncodeMomentBatch(xStream, yStream)

		So(tests.WriteSamples(process, batch...), ShouldBeNil)

		outbound, err := readOutbound(process)

		So(err, ShouldBeNil)

		Convey("It should report high moment-fit confidence", func() {
			So(datura.Peek[float64](outbound, "output", "confidence"), ShouldBeGreaterThan, 0.5)
		})
	})
}

func BenchmarkHawkes_Observe(testingTB *testing.B) {
	xStream := []float64{2, 4, 6, 8, 10, 12}
	yStream := []float64{1, 2, 3, 4, 5, 6}
	params := hawkes.BivariateParams{
		MuX:     5,
		MuY:     3,
		AlphaXX: 0.1,
		AlphaYY: 0.1,
		Beta:    1,
	}
	process := NewHawkes(hawkesMomentConfig(params, 1, 1))
	batch := hawkes.EncodeMomentBatch(xStream, yStream)
	frame := make([]byte, 4096)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = tests.WriteSamples(process, batch...)
		_, _ = process.Read(frame)
	}
}
