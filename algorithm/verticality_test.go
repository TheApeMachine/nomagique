package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestNewVerticality(testingTB *testing.T) {
	Convey("Given extracted verticality features on the artifact", testingTB, func() {
		verticality := NewVerticality()

		So(verticality, ShouldNotBeNil)

		artifact := datura.Acquire("verticality-test", datura.APPJSON).
			WithPayload([]byte(`{"features":[4.0,0.8,0.2,0.05]}`))

		err := transport.NewFlipFlop(artifact, verticality)

		So(err, ShouldBeNil)

		Convey("It should publish category scores on output", func() {
			So(datura.Peek[float64](artifact, "output", "ignitionScore"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkVerticalityRead(b *testing.B) {
	verticality := NewVerticality()
	artifact := datura.Acquire("verticality-bench", datura.APPJSON).
		WithPayload([]byte(`{"features":[4.0,0.8,0.2,0.05]}`))

	b.ReportAllocs()

	for b.Loop() {
		_ = transport.NewFlipFlop(artifact, verticality)
	}
}
