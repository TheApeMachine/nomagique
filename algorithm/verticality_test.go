package algorithm

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/nomagique/tests"
)

func TestVerticalityEvaluate(testingTB *testing.T) {
	Convey("Given a volume spike with rising price", testingTB, func() {
		verticality, err := NewVerticality()
		So(err, ShouldBeNil)

		writeErr := tests.WriteSamples(verticality, 4.0, 0.8, 0.2, 0.05)
		So(writeErr, ShouldBeNil)
		_, _ = verticality.Read(make([]byte, 4096))

		Convey("It should publish an eligible verticality outcome", func() {
			So(verticality.outcome.Eligible, ShouldBeTrue)
			So(verticality.outcome.Strength, ShouldBeGreaterThan, 0)
			So(verticality.outcome.Category, ShouldBeGreaterThan, 0)
		})
	})
}

func BenchmarkVerticalityRead(b *testing.B) {
	verticality, err := NewVerticality()

	if err != nil {
		b.Fatal(err)
	}

	samples := []float64{4.0, 0.8, 0.2, 0.05}

	b.ReportAllocs()

	for b.Loop() {
		_ = tests.WriteSamples(verticality, samples...)
		_, _ = verticality.Read(make([]byte, 4096))
	}
}
