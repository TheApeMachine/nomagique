package equation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestVerticalityRead(testingTB *testing.T) {
	Convey("Given extracted ticker features on the artifact", testingTB, func() {
		verticality := NewVerticality()

		So(verticality, ShouldNotBeNil)

		artifact := datura.Acquire("verticality-test", datura.APPJSON).
			WithPayload([]byte(`{"features":[11000,10000,41000,40990,41010,3.1]}`))

		err := transport.NewFlipFlop(artifact, verticality)

		So(err, ShouldBeNil)

		Convey("It should publish category scores on output", func() {
			So(datura.Peek[float64](artifact, "output", "ignition"), ShouldBeGreaterThan, 0)
			So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkVerticalityRead(b *testing.B) {
	verticality := NewVerticality()
	artifact := datura.Acquire("verticality-bench", datura.APPJSON).
		WithPayload([]byte(`{"features":[11000,10000,41000,40990,41010,3.1]}`))

	b.ReportAllocs()

	for b.Loop() {
		_ = transport.NewFlipFlop(artifact, verticality)
	}
}
