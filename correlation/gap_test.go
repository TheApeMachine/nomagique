package correlation

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

func gapConfig() *datura.Artifact {
	return datura.Acquire("gap-config", datura.APPJSON).
		Poke(1.0, "config", "maxIntervalSeconds")
}

func coupledGapBatch() []float64 {
	syncLeft := []float64{1, 2, 3, 4, 5, 6}
	syncRight := []float64{2, 4, 6, 8, 10, 12}
	asyncLeft := []float64{0, 100, 1, 110, 2, 121, 3, 133.1}
	asyncRight := []float64{0, 50, 1, 55, 2, 60.5, 3, 66.55}

	batch := make(
		[]float64,
		0,
		2+len(syncLeft)+len(syncRight)+len(asyncLeft)+len(asyncRight),
	)
	batch = append(batch, float64(len(syncLeft)), float64(len(asyncLeft)/2))
	batch = append(batch, syncLeft...)
	batch = append(batch, syncRight...)
	batch = append(batch, asyncLeft...)
	batch = append(batch, asyncRight...)

	return batch
}

func TestGap_Read(testingTB *testing.T) {
	Convey("Given config on the constructor artifact and a coupled gap batch on inbound wire", testingTB, func() {
		stage := NewGap(gapConfig())
		artifact := datura.Acquire("gap-inbound", datura.APPJSON).
			Poke(coupledGapBatch(), "batch")
		err := nomagique.RoundTripArtifact(artifact, stage)

		So(err, ShouldBeNil)

		rootKey := datura.Peek[string](artifact, "root")
		inputs := datura.Peek[[]string](artifact, "inputs")
		pearson := datura.Peek[float64](artifact, rootKey, "pearson")
		hayashi := datura.Peek[float64](artifact, rootKey, "hayashi")
		gap := datura.Peek[float64](artifact, rootKey, "gap")

		Convey("It should publish root and inputs for downstream navigation", func() {
			So(rootKey, ShouldEqual, "output")
			So(
				inputs,
				ShouldResemble,
				[]string{"value", "pearson", "hayashi", "gap"},
			)
		})

		Convey("It should expose pearson and hayashi under root inputs", func() {
			So(pearson, ShouldBeGreaterThan, 0.9)
			So(hayashi, ShouldAlmostEqual, 1, 1e-6)
			So(gap, ShouldEqual, hayashi-pearson)
		})
	})

	Convey("Given features payload from a coupled gap batch", testingTB, func() {
		stage := NewGap(gapConfig())
		artifact := datura.Acquire("gap-inbound", datura.APPJSON).
			Poke(coupledGapBatch(), "batch")
		err := nomagique.RoundTripArtifact(artifact, stage)

		So(err, ShouldBeNil)

		rootKey := datura.Peek[string](artifact, "root")

		Convey("It should resolve pearson via root and inputs", func() {
			So(rootKey, ShouldEqual, "output")
			So(datura.Peek[float64](artifact, rootKey, "pearson"), ShouldBeGreaterThan, 0.9)
			So(datura.Peek[float64](artifact, rootKey, "hayashi"), ShouldAlmostEqual, 1, 1e-6)
		})
	})
}

func BenchmarkGap_Read(testingTB *testing.B) {
	stage := NewGap(gapConfig())
	artifact := datura.Acquire("gap-inbound", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		artifact.Poke(coupledGapBatch(), "batch")
		_ = nomagique.RoundTripArtifact(artifact, stage)
	}
}
