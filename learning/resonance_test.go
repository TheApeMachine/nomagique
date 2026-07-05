package learning

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"gonum.org/v1/gonum/mat"
)

func TestNewResonanceManifold(testingTB *testing.T) {
	Convey("Given a valid architecture", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{4, 8, 4}, 2, 0.01)

		Convey("It should construct a usable manifold", func() {
			So(err, ShouldBeNil)
			So(manifold, ShouldNotBeNil)
			So(manifold.streamLearn, ShouldBeTrue)
			So(manifold.streamAdvanceTemporal, ShouldBeTrue)
		})
	})

	Convey("Given invalid alpha", testingTB, func() {
		_, err := NewResonanceManifold([]int{4, 8, 4}, 2, 0)

		Convey("It should return an error", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func TestAdaptiveResonanceConfig(testingTB *testing.T) {
	Convey("Given alpha and depth", testingTB, func() {
		derived := AdaptiveResonanceConfig(0.01, []int{4, 8, 4})

		Convey("It should derive mix, patience, and clip from alpha and depth", func() {
			So(derived.TemporalWeight, ShouldBeGreaterThan, 0)
			So(derived.TopDownInitMix, ShouldBeGreaterThan, 0)
			So(derived.EarlyStopPatience, ShouldBeGreaterThan, 0)
			So(derived.GradClip, ShouldBeGreaterThan, 0)
			So(derived.StateClip, ShouldBeGreaterThan, 0)
		})
	})
}

func TestResonanceManifoldSettleAdvanceTemporal(testingTB *testing.T) {
	Convey("Given inference without learning", testingTB, func() {
		architecture := []int{4, 8, 4}
		manifold, err := NewResonanceManifold(architecture, 0, 0.05)
		So(err, ShouldBeNil)

		firstInput := []float64{0.8, -0.2, 0.4, 0.1}
		secondInput := []float64{-0.3, 0.6, -0.1, 0.2}

		err = manifold.Settle(firstInput, true)
		So(err, ShouldBeNil)

		withHistoryErr := manifold.Settle(secondInput, true)
		So(withHistoryErr, ShouldBeNil)
		withHistory := manifold.LatentState()

		coldStart, err := NewResonanceManifold(architecture, 0, 0.05)
		So(err, ShouldBeNil)

		coldErr := coldStart.Settle(secondInput, false)
		So(coldErr, ShouldBeNil)
		coldLatent := coldStart.LatentState()

		Convey("It should keep temporal priors active without Learn", func() {
			So(withHistory, ShouldNotResemble, coldLatent)
		})
	})
}

func TestResonanceManifoldSetStreamLearn(testingTB *testing.T) {
	Convey("Given a manifold with learning disabled on the stream path", testingTB, func() {
		architecture := []int{2, 4, 2}
		input := []float64{0.3, -0.7}
		target := []float64{0.9}

		baseline, err := NewResonanceManifold(architecture, 1, 0.03)
		So(err, ShouldBeNil)

		frozenManifold, err := NewResonanceManifold(architecture, 1, 0.03)
		So(err, ShouldBeNil)
		frozenManifold.W[0].Copy(baseline.W[0])
		frozenManifold.R[0].Copy(baseline.R[0])
		frozenManifold.A.Copy(baseline.A)
		frozenManifold.V.Copy(baseline.V)

		baselineWeights := mat.DenseCopyOf(baseline.W[0])

		_, _ = baseline.SettleFromBatchOptions(input, target, true, true)
		frozenManifold.SetStreamLearn(false)
		_, _ = frozenManifold.SettleFromBatchOptions(input, target, false, true)

		Convey("It should leave weights unchanged when learning is disabled", func() {
			So(mat.Equal(baselineWeights, frozenManifold.W[0]), ShouldBeTrue)
			So(mat.Equal(baselineWeights, baseline.W[0]), ShouldBeFalse)
		})
	})
}

func TestResonanceManifoldDirectBatch(testingTB *testing.T) {
	Convey("Given stream input with a supervised target", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{2, 4, 2}, 1, 0.02)
		So(err, ShouldBeNil)

		got, err := manifold.SettleFromBatch([]float64{0.2, -0.4}, []float64{0.8})
		latent := manifold.LatentState()

		Convey("It should expose reconstruction and latent state directly", func() {
			So(err, ShouldBeNil)
			So(math.IsNaN(got), ShouldBeFalse)
			So(len(latent), ShouldEqual, 2)
		})
	})
}

func BenchmarkResonanceManifoldSettle(testingTB *testing.B) {
	manifold, err := NewResonanceManifold([]int{8, 16, 8}, 2, 0.01)

	if err != nil {
		testingTB.Fatal(err)
	}

	input := []float64{0.1, -0.2, 0.3, -0.4, 0.5, -0.6, 0.7, -0.8}
	target := []float64{0.25, -0.5}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := manifold.SettleFromBatch(input, target); err != nil {
			testingTB.Fatal(err)
		}
	}
}
