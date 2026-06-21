package learning

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
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
}

func TestResonanceManifold_SettleAdvanceTemporal(testingTB *testing.T) {
	Convey("Given inference without learning", testingTB, func() {
		architecture := []int{4, 8, 4}
		manifold, err := NewResonanceManifold(architecture, 0, 0.05)
		So(err, ShouldBeNil)

		firstInput := []float64{0.8, -0.2, 0.4, 0.1}
		secondInput := []float64{-0.3, 0.6, -0.1, 0.2}

		settleErr := manifold.Settle(firstInput, true)
		So(settleErr, ShouldBeNil)

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

func TestResonanceManifold_SupervisedTargetDoesNotContaminateSettle(testingTB *testing.T) {
	Convey("Given the same input and settled state", testingTB, func() {
		architecture := []int{3, 6, 3}
		input := []float64{0.5, -0.25, 0.75}
		target := []float64{1.0, -1.0}

		unlabeled, err := NewResonanceManifold(architecture, 2, 0.02)
		So(err, ShouldBeNil)
		So(unlabeled.Settle(input, false), ShouldBeNil)
		unlabeledEnergy := unlabeled.Energy()
		unlabeledLatent := unlabeled.LatentState()

		labeled, err := NewResonanceManifold(architecture, 2, 0.02)
		So(err, ShouldBeNil)
		So(labeled.Settle(input, false), ShouldBeNil)
		labeledEnergy := labeled.Energy()
		labeledLatent := labeled.LatentState()
		labeled.Learn(target)

		Convey("Settle should ignore supervised targets during inference", func() {
			So(unlabeledEnergy, ShouldAlmostEqual, labeledEnergy, 1e-12)
			So(unlabeledLatent, ShouldResemble, labeledLatent)
		})
	})
}

func TestResonanceManifold_SetStreamLearn(testingTB *testing.T) {
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

		baseline.SettleFromBatchOptions(input, target, true, true)
		frozenManifold.SetStreamLearn(false)
		frozenManifold.SettleFromBatchOptions(input, target, false, true)

		Convey("It should leave weights unchanged when learning is disabled", func() {
			So(mat.Equal(baselineWeights, frozenManifold.W[0]), ShouldBeTrue)
			So(mat.Equal(baselineWeights, baseline.W[0]), ShouldBeFalse)
		})
	})
}

func TestResonanceManifold_Read(testingTB *testing.T) {
	Convey("Given stream input with a supervised target", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{2, 4, 2}, 1, 0.02)
		So(err, ShouldBeNil)

		artifact := datura.Acquire("test", datura.APPJSON).
			Poke([]float64{0.2, -0.4, 0.8}, "batch")
		err = transport.NewFlipFlop(artifact, manifold)

		So(err, ShouldBeNil)

		got := datura.Peek[float64](artifact, "output", "value")
		latent := datura.Peek[[]float64](artifact, "latent")

		Convey("It should expose reconstruction and latent state on the artifact", func() {
			So(math.IsNaN(got), ShouldBeFalse)
			So(len(latent), ShouldEqual, 2)
			So(len(manifold.LatentState()), ShouldEqual, 2)
		})
	})
}

func TestResonanceManifold_WireSnapshot(testingTB *testing.T) {
	Convey("Given a settled resonance manifold", testingTB, func() {
		architecture := []int{4, 8, 3}
		manifold, err := NewResonanceManifold(architecture, 0, 0.02)

		So(err, ShouldBeNil)
		So(manifold.Settle([]float64{50000, 0.02, 1200, 0.015}, true), ShouldBeNil)

		layers, surprise, energy := manifold.WireSnapshot()

		Convey("It should export finite layer states and errors", func() {
			So(len(layers), ShouldEqual, len(architecture))

			totalRows := 0

			for layerIndex, layer := range layers {
				So(len(layer.State), ShouldEqual, architecture[layerIndex])
				So(len(layer.Prediction), ShouldEqual, architecture[layerIndex])
				So(surprise, ShouldBeGreaterThanOrEqualTo, 0)
				So(energy, ShouldBeGreaterThanOrEqualTo, 0)

				for _, value := range layer.State {
					So(math.IsNaN(value), ShouldBeFalse)
					So(math.IsInf(value, 0), ShouldBeFalse)
				}

				if layerIndex < len(architecture)-1 {
					So(layer.ErrorNorm, ShouldBeGreaterThanOrEqualTo, 0)
				}

				totalRows += len(layer.State)
			}

			So(totalRows, ShouldEqual, 15)
		})
	})
}

func TestResonanceManifold_Sparsity(testingTB *testing.T) {
	Convey("Given a non-zero latent state", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{2, 3, 2}, 0, 0.02)
		So(err, ShouldBeNil)
		So(manifold.Settle([]float64{0.5, -0.5}, false), ShouldBeNil)

		withSparsity := manifold.cfg.Sparsity
		manifold.cfg.Sparsity = 0
		energyWithout := manifold.Energy()
		manifold.cfg.Sparsity = withSparsity
		energyWith := manifold.Energy()

		Convey("It should apply the configured sparsity penalty", func() {
			So(energyWith, ShouldBeGreaterThan, energyWithout)
		})
	})
}

func BenchmarkResonanceManifold_Settle(testingTB *testing.B) {
	manifold, err := NewResonanceManifold([]int{8, 16, 8}, 2, 0.01)

	if err != nil {
		testingTB.Fatal(err)
	}

	input := []float64{0.1, -0.2, 0.3, -0.4, 0.5, -0.6, 0.7, -0.8}
	target := []float64{0.25, -0.5}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = manifold.Settle(input, true)
		manifold.Learn(target)
	}
}
