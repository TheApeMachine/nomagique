package learning

import (
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"gonum.org/v1/gonum/mat"
)

/*
TestTemporalErrorIsMeasured pins that the top layer reports a real temporal
residual rather than a structural zero.

The top latent is not predicted top-down by any weight matrix, because there is
one fewer link than there are layers. WireSnapshot indexed layer errors by layer
and so left the top layer's error norm at its zero value, which a controller
reading it interpreted as perfect temporal prediction on every tick.
*/
func TestTemporalErrorIsMeasured(testingTB *testing.T) {
	Convey("Given a manifold that has settled more than once", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{4, 8, 4}, 1, 0.03)
		So(err, ShouldBeNil)

		_, hasBefore := manifold.TemporalError()

		for range 5 {
			_, err := manifold.SettleFromBatch(
				[]float64{0.8, -0.2, 0.4, 0.1}, []float64{0.0004},
			)
			So(err, ShouldBeNil)
		}

		temporal, hasAfter := manifold.TemporalError()
		layers, _, _ := manifold.WireSnapshot()

		Convey("Then the temporal residual is undefined before any prior state", func() {
			So(hasBefore, ShouldBeFalse)
		})

		Convey("Then the top layer reports that residual instead of zero", func() {
			So(hasAfter, ShouldBeTrue)
			So(temporal, ShouldBeGreaterThan, 0)

			top := layers[len(layers)-1]
			So(top.Temporal, ShouldBeTrue)
			So(top.ErrorNorm, ShouldEqual, temporal)
		})
	})
}

/*
TestSetAlphaPreservesGeometry pins the split between learning pace and state
geometry.

Alpha previously re-derived the whole config, including StateClip, which was
inversely coupled to it. A controller moving alpha across its range therefore
tightened the admissible latent range by a factor of thirty, changing the state
space that the retained weights and precisions had been estimated in.
*/
func TestSetAlphaPreservesGeometry(testingTB *testing.T) {
	Convey("Given a manifold whose pace is retuned across its range", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{4, 8, 4}, 1, 0.005)
		So(err, ShouldBeNil)

		before := manifold.cfg

		manifold.SetAlpha(0.150)
		after := manifold.cfg

		Convey("Then the geometry of the state space is unchanged", func() {
			So(after.StateClip, ShouldEqual, before.StateClip)
			So(after.TopDownInitMix, ShouldEqual, before.TopDownInitMix)
			So(after.MaxInferenceSteps, ShouldEqual, before.MaxInferenceSteps)
			So(after.MinInferenceSteps, ShouldEqual, before.MinInferenceSteps)
			So(after.EarlyStopPatience, ShouldEqual, before.EarlyStopPatience)
		})

		Convey("Then the learning pace does move", func() {
			So(after.LrGenerative, ShouldBeGreaterThan, before.LrGenerative)
			So(after.LrState, ShouldBeGreaterThan, before.LrState)
			So(after.PrecisionBeta, ShouldBeGreaterThan, before.PrecisionBeta)
		})
	})

	Convey("Given the derived config at either end of the pace range", testingTB, func() {
		slow := AdaptiveResonanceConfig(0.005, []int{4, 8, 4})
		fast := AdaptiveResonanceConfig(0.150, []int{4, 8, 4})

		Convey("Then the state clip does not depend on the pace at all", func() {
			So(slow.StateClip, ShouldEqual, fast.StateClip)
		})
	})
}

/*
TestTaskHeadFitsSmallTargets pins that the supervised head learns a target on
the scale of a log return.

A tanh output head aimed at a target of order 1e-4 has a local derivative of
essentially one and an error of essentially zero, so it learns almost nothing
per sample while weight decay pulls it toward the origin. The head ends up
forecasting zero, which reads downstream as a confident no-edge.
*/
func TestTaskHeadFitsSmallTargets(testingTB *testing.T) {
	Convey("Given a target on the scale of a per-tick log return", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{4, 8, 4}, 1, 0.05)
		So(err, ShouldBeNil)

		/*
			A fixed input paired with a fixed target is the most favourable case
			the head could be given. If it cannot fit this, it cannot fit
			anything.
		*/
		input := []float64{0.8, -0.2, 0.4, 0.1}
		const target = 4e-4

		for range 400 {
			So(manifold.Settle(input, false), ShouldBeNil)
			So(manifold.Learn([]float64{target}), ShouldBeNil)
		}

		So(manifold.Settle(input, false), ShouldBeNil)
		prediction := manifold.TaskPrediction()

		testingTB.Logf("target %.3e predicted %.3e", target, prediction[0])

		Convey("Then the head moves toward the target rather than to zero", func() {
			So(len(prediction), ShouldEqual, 1)
			So(prediction[0], ShouldBeGreaterThan, 0)

			/*
				Within a factor of two of a target four orders of magnitude
				inside tanh's linear region. The head this replaced stayed
				pinned within noise of zero for the same schedule.
			*/
			So(prediction[0], ShouldBeBetween, target/2, target*2)
		})
	})

	Convey("Given a head that has seen no samples", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{4, 8, 4}, 1, 0.05)
		So(err, ShouldBeNil)
		So(manifold.Settle([]float64{0.8, -0.2, 0.4, 0.1}, false), ShouldBeNil)

		Convey("Then it forecasts nothing rather than a random draw", func() {
			So(manifold.TaskPrediction()[0], ShouldEqual, 0)
		})
	})
}

/*
TestRolloutRetentionReportsDecay pins that the rollout's contraction is
observable rather than silently folded into the forecast.

The recursion z <- tanh(A z) relaxes toward the origin, so later curve entries
carry the decay envelope rather than a statement about the market. A consumer
reading the whole curve cannot tell the two apart without the envelope.
*/
func TestRolloutRetentionReportsDecay(testingTB *testing.T) {
	Convey("Given a settled manifold rolled forward", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{4, 8, 4}, 1, 0.03)
		So(err, ShouldBeNil)

		for range 20 {
			_, err := manifold.SettleFromBatch(
				[]float64{0.8, -0.2, 0.4, 0.1}, []float64{0.0004},
			)
			So(err, ShouldBeNil)
		}

		retention := manifold.RolloutRetention(12)
		curve := manifold.RolloutTaskPrediction(12)

		testingTB.Logf("retention %.4f -> %.4f over %d steps",
			retention[0], retention[len(retention)-1], len(retention))

		Convey("Then retention is reported per step alongside the curve", func() {
			So(len(retention), ShouldEqual, 12)
			So(len(curve), ShouldEqual, 12)
			So(retention[0], ShouldEqual, 1)
		})

		Convey("Then retention decays rather than holding flat", func() {
			So(retention[0], ShouldBeGreaterThan, 0)
			So(retention[len(retention)-1], ShouldBeLessThan, retention[0])

			for _, surviving := range retention {
				So(surviving, ShouldBeGreaterThanOrEqualTo, 0)
				So(math.IsNaN(surviving), ShouldBeFalse)
			}
		})
	})
}

/*
TestPredictionEnergyExcludesRegularizers pins that the reported energy responds
only to prediction quality.

Energy is the objective the inference line search minimizes, so it correctly
includes the latent decay and sparsity penalties. Those penalty magnitudes are
set by alpha, which means reporting Energy as a measure of prediction quality
makes the number move whenever the pace is retuned.
*/
func TestPredictionEnergyExcludesRegularizers(testingTB *testing.T) {
	Convey("Given a settled manifold whose pace is retuned", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{4, 8, 4}, 1, 0.005)
		So(err, ShouldBeNil)

		for range 10 {
			_, err := manifold.SettleFromBatch(
				[]float64{0.8, -0.2, 0.4, 0.1}, []float64{0.0004},
			)
			So(err, ShouldBeNil)
		}

		predictionBefore := manifold.PredictionEnergy()
		energyBefore := manifold.Energy()

		manifold.SetAlpha(0.150)

		predictionAfter := manifold.PredictionEnergy()
		energyAfter := manifold.Energy()

		testingTB.Logf("prediction %.6f -> %.6f, total %.6f -> %.6f",
			predictionBefore, predictionAfter, energyBefore, energyAfter)

		Convey("Then prediction energy is invariant to the pace", func() {
			So(predictionAfter, ShouldAlmostEqual, predictionBefore, 1e-12)
		})

		Convey("Then the full objective does move, which is why it is not reported", func() {
			So(energyAfter, ShouldNotAlmostEqual, energyBefore, 1e-12)
		})

		Convey("Then the objective is the prediction term plus the penalties", func() {
			So(energyBefore, ShouldBeGreaterThanOrEqualTo, predictionBefore)
		})
	})
}

/*
TestTaskPrecisionIsScaleFree pins that the task precision expresses per-row
reliability rather than saturating on the caller's choice of target scale.

An absolute inverse variance on a residual of order 1e-4 is 1e8, which the
clamp truncates to PrecisionMax on every row of every tick, discarding exactly
the information the term exists to carry.
*/
func TestTaskPrecisionIsScaleFree(testingTB *testing.T) {
	Convey("Given identical dynamics fit across six orders of target scale", testingTB, func() {
		/*
			The head converges over several hundred samples, and during that
			convergence the residual variance is still falling toward its
			resting level. Precision is only scale-free once it has settled, so
			the comparison has to be made on converged fits.
		*/
		fit := func(scale float64) (float64, float64) {
			manifold, err := NewResonanceManifold([]int{4, 8, 4}, 1, 0.05)
			So(err, ShouldBeNil)

			for step := range 600 {
				So(manifold.Settle([]float64{0.8, -0.2, 0.4, 0.1}, false), ShouldBeNil)
				So(manifold.Learn([]float64{
					scale * math.Sin(float64(step)*0.3),
				}), ShouldBeNil)
			}

			return manifold.taskPrecision.At(0, 0), manifold.taskVar.At(0, 0)
		}

		scales := []float64{1e-6, 1e-4, 1e-2, 1e-1, 1.0}
		precisions := make([]float64, 0, len(scales))
		variances := make([]float64, 0, len(scales))

		for _, scale := range scales {
			precision, variance := fit(scale)
			precisions = append(precisions, precision)
			variances = append(variances, variance)

			testingTB.Logf("target scale %.0e -> variance %.3e, precision %.4f",
				scale, variance, precision)
		}

		Convey("Then the residual variance really does span those orders", func() {
			/*
				Without this the comparison below would be vacuous: precisions
				agreeing across scales means nothing unless the underlying
				variances actually differed.
			*/
			So(variances[len(variances)-1]/variances[0], ShouldBeGreaterThan, 1e9)
		})

		Convey("Then no scale pins the precision at either clamp", func() {
			for _, precision := range precisions {
				So(precision, ShouldBeLessThan, 5.0)
				So(precision, ShouldBeGreaterThan, 0.10)
			}
		})

		Convey("Then every scale reports comparable reliability", func() {
			/*
				The residual sequences differ only by a constant factor, so a
				scale-free precision must land in the same neighbourhood at
				every one. An absolute inverse variance would differ by nine
				orders of magnitude before clamping, and be identically pinned
				at the clamp after.
			*/
			lowest := precisions[0]
			highest := precisions[0]

			for _, precision := range precisions {
				lowest = math.Min(lowest, precision)
				highest = math.Max(highest, precision)
			}

			So(highest/lowest, ShouldBeLessThan, 1.5)
		})
	})
}

/*
TestSettleRemainsFiniteAcrossPaceChanges pins that retuning the pace mid-stream
does not destabilize inference.

This is the composite guard on the pace and geometry split: a controller is free
to move alpha across its whole range on live data, so every reading the network
produces has to stay finite while it does.
*/
func TestSettleRemainsFiniteAcrossPaceChanges(testingTB *testing.T) {
	Convey("Given a manifold driven while its pace is swept", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{6, 12, 6}, 1, 0.03)
		So(err, ShouldBeNil)

		paces := []float64{0.005, 0.150, 0.005, 0.150, 0.03}
		input := make([]float64, 6)

		for step := range 500 {
			manifold.SetAlpha(paces[step%len(paces)])

			for index := range input {
				input[index] = math.Sin(float64(step+index) * 0.17)
			}

			surprise, err := manifold.SettleFromBatch(input, []float64{
				2e-4 * math.Cos(float64(step)*0.11),
			})

			So(err, ShouldBeNil)
			So(math.IsNaN(surprise), ShouldBeFalse)
			So(math.IsInf(surprise, 0), ShouldBeFalse)
		}

		Convey("Then every retained matrix stays finite", func() {
			for _, weights := range manifold.W {
				So(finiteDense(weights), ShouldBeTrue)
			}

			So(finiteDense(manifold.A), ShouldBeTrue)
			So(finiteDense(manifold.V), ShouldBeTrue)
		})
	})
}

func finiteDense(matrix *mat.Dense) bool {
	rows, cols := matrix.Dims()

	for rowIndex := range rows {
		for colIndex := range cols {
			if !finite(matrix.At(rowIndex, colIndex)) {
				return false
			}
		}
	}

	return true
}
