package equation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFeatureFrame_FeatureFields(testingTB *testing.T) {
	Convey("Given a typed feature frame", testingTB, func() {
		frame := NewFeatureFrame(
			[]string{"alpha", "beta", "gamma"},
			[]float64{1, 2, 3},
		)

		values, err := frame.FeatureFields([]string{"gamma", "alpha"})

		Convey("It should read fields in requested order", func() {
			So(err, ShouldBeNil)
			So(values, ShouldResemble, []float64{3, 1})
		})

		Convey("It should reject missing schema keys", func() {
			_, err := frame.FeatureFields([]string{"missing"})

			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkFeatureFrameFeatureFields(b *testing.B) {
	frame := NewFeatureFrame(
		[]string{"alpha", "beta", "gamma"},
		[]float64{1, 2, 3},
	)

	b.ReportAllocs()

	for b.Loop() {
		_, _ = frame.FeatureFields([]string{"gamma", "alpha"})
	}
}
