package vector

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func TestFeatureSampleRead(t *testing.T) {
	Convey("Given a feature vector", t, func() {
		config := datura.Acquire("feature-sample-config", datura.APPJSON).
			Poke(1.0, "featureIndex")
		stage := NewFeatureSample(config)
		frame := datura.Acquire("feature-sample-frame", datura.APPJSON)
		frame.Merge("features", []float64{10, 20, 30})

		err := nomagique.RoundTripArtifact(frame, stage)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](frame, "output", "value"), ShouldEqual, 20)
		So(datura.Peek[string](frame, "root"), ShouldEqual, "output")
	})

	Convey("Given an out-of-range feature index", t, func() {
		config := datura.Acquire("feature-sample-config", datura.APPJSON).
			Poke(5.0, "featureIndex")
		stage := NewFeatureSample(config)
		frame := datura.Acquire("feature-sample-frame", datura.APPJSON)
		frame.Merge("features", []float64{10, 20})

		err := nomagique.RoundTripArtifact(frame, stage)

		So(err, ShouldNotBeNil)
	})
}

func BenchmarkFeatureSampleRead(testingTB *testing.B) {
	config := datura.Acquire("feature-sample-config", datura.APPJSON).
		Poke(1.0, "featureIndex")
	stage := NewFeatureSample(config)
	frame := datura.Acquire("feature-sample-frame", datura.APPJSON)
	frame.Merge("features", []float64{10, 20, 30})

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = nomagique.RoundTripArtifact(frame, stage)
	}
}
