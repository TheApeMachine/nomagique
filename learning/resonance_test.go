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
			So(derived.TemporalNormMax, ShouldBeBetween, 0, 1)
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

	Convey("Given batch learning with temporal advancement enabled", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{1, 1}, 0, 0.05)
		So(err, ShouldBeNil)
		manifold.cfg.LrTemporal = 1
		manifold.cfg.TemporalWeight = 1
		manifold.cfg.WeightDecay = 0
		manifold.cfg.GradClip = 1
		manifold.cfg.TemporalNormMax = 0.99

		So(manifold.Settle([]float64{0.2}, false), ShouldBeNil)
		So(manifold.Learn(nil), ShouldBeNil)
		previousTop := manifold.z[1].At(0, 0)
		manifold.A.Zero()

		_, err = manifold.SettleFromBatchOptions(
			[]float64{-0.3},
			nil,
			true,
			true,
		)
		So(err, ShouldBeNil)
		currentTop := manifold.z[1].At(0, 0)
		temporalError, hasTemporal := manifold.TemporalError()
		layers, _, _ := manifold.WireSnapshot()

		Convey("It should learn the transition from the previous top state", func() {
			So(manifold.A.At(0, 0), ShouldAlmostEqual, currentTop*previousTop, 1e-12)
			So(manifold.prevTop.At(0, 0), ShouldAlmostEqual, currentTop, 1e-12)
		})

		Convey("It should retain the temporal prediction used during inference", func() {
			So(hasTemporal, ShouldBeTrue)
			So(temporalError, ShouldAlmostEqual, math.Abs(currentTop), 1e-12)
			So(layers[1].Prediction[0], ShouldEqual, 0)
			So(layers[1].ErrorNorm, ShouldAlmostEqual, temporalError, 1e-12)
		})
	})

	Convey("Given temporal state advanced before a learning call", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{1, 1}, 0, 0.05)
		So(err, ShouldBeNil)
		So(manifold.Settle([]float64{0.2}, true), ShouldBeNil)

		Convey("It should reject a self-transition update", func() {
			So(manifold.Learn(nil), ShouldNotBeNil)
		})
	})

	Convey("Given accepted stable inference steps", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{1, 1}, 0, 0.05)
		So(err, ShouldBeNil)
		manifold.cfg.LrState = 0
		manifold.cfg.MonotoneStateSteps = false
		manifold.cfg.MinInferenceSteps = 1
		manifold.cfg.MaxInferenceSteps = 6
		manifold.cfg.EarlyStopPatience = 3

		So(manifold.Settle([]float64{0.2}, false), ShouldBeNil)

		Convey("It should require the configured consecutive evidence", func() {
			So(manifold.lastInferenceSteps, ShouldEqual, 3)
		})
	})

	Convey("Given line-search proposals that are rejected", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{1, 1}, 0, 0.05)
		So(err, ShouldBeNil)
		manifold.cfg.LrState = -100
		manifold.cfg.MonotoneStateSteps = true
		manifold.cfg.LineSearchHalvings = 0
		manifold.cfg.MinInferenceSteps = 1
		manifold.cfg.MaxInferenceSteps = 4
		manifold.cfg.EarlyStopPatience = 1

		So(manifold.Settle([]float64{0.2}, false), ShouldBeNil)

		Convey("It should not mistake rejection for convergence", func() {
			So(manifold.lastInferenceSteps, ShouldEqual, manifold.cfg.MaxInferenceSteps)
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

func TestResonanceManifoldLearn(testingTB *testing.T) {
	Convey("Given covariance-derived and fixed-rate task updates", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{1, 1}, 1, 0.03)
		So(err, ShouldBeNil)

		fixedIntercept := 0.0
		fixedWeight := 0.0
		fixedRate := 0.03
		adaptiveLoss := 0.0
		fixedLoss := 0.0

		/*
			Repeatedly traversing both signs makes slope and intercept identifiable.
			The fixed comparator gets the same slope-and-intercept form so the
			difference isolates gain selection; its rate is the scalar pace the task
			head previously shared with the manifold. RLS must lower prior, not
			post-fit, loss.
		*/
		for sampleIndex := range 128 {
			feature := float64(sampleIndex%8-4) / 4
			target := 0.25 + 0.5*feature
			manifold.z[len(manifold.z)-1].Set(0, 0, feature)

			adaptivePrediction := manifold.TaskPrediction()[0]
			adaptiveError := target - adaptivePrediction
			adaptiveLoss += adaptiveError * adaptiveError

			fixedPrediction := fixedIntercept + fixedWeight*feature
			fixedError := target - fixedPrediction
			fixedLoss += fixedError * fixedError
			fixedIntercept += fixedRate * fixedError
			fixedWeight += fixedRate * fixedError * feature

			So(manifold.Learn([]float64{target}), ShouldBeNil)
		}

		manifold.z[len(manifold.z)-1].Set(0, 0, 0.5)
		prediction := manifold.TaskPrediction()[0]

		Convey("It should derive gain from design covariance and lower prequential error", func() {
			So(adaptiveLoss, ShouldBeLessThan, fixedLoss)
			So(prediction, ShouldAlmostEqual, 0.5, 0.01)
		})
	})

	Convey("Given a supervised sample before its RLS update", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{1, 1}, 1, 1)
		So(err, ShouldBeNil)
		manifold.z[1].Set(0, 0, 0.5)

		So(manifold.Learn([]float64{1}), ShouldBeNil)

		Convey("It should retain the strictly prior forecast error", func() {
			So(manifold.taskVar.At(0, 0), ShouldEqual, 1)
		})
	})

	Convey("Given an explicitly malformed supervised target", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{1, 1}, 1, 0.05)
		So(err, ShouldBeNil)

		Convey("It should distinguish it from an absent target", func() {
			So(manifold.Learn(nil), ShouldBeNil)
			So(manifold.Learn([]float64{}), ShouldNotBeNil)
			So(manifold.Learn([]float64{1, 2}), ShouldNotBeNil)
		})
	})

	Convey("Given exact zero residuals at unit precision pace", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{1, 1}, 1, 1)
		So(err, ShouldBeNil)

		So(manifold.Learn([]float64{0}), ShouldBeNil)
		So(manifold.Settle([]float64{0}, false), ShouldBeNil)
		So(manifold.Learn([]float64{0}), ShouldBeNil)

		Convey("It should floor bounded-state variances without inventing target scale", func() {
			So(manifold.errorVar[0].At(0, 0), ShouldEqual, manifold.cfg.PrecisionEps)
			So(manifold.temporalVar.At(0, 0), ShouldEqual, manifold.cfg.PrecisionEps)
			_, hasTaskPrecision := manifold.TaskPrecision()
			So(hasTaskPrecision, ShouldBeFalse)
		})
	})

	Convey("Given learned temporal weights with excessive operator norm", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{2, 2}, 0, 0.05)
		So(err, ShouldBeNil)
		So(manifold.Settle([]float64{0.2, -0.1}, false), ShouldBeNil)
		So(manifold.Learn(nil), ShouldBeNil)
		manifold.A.Set(0, 0, 10)
		manifold.A.Set(0, 1, 4)
		manifold.A.Set(1, 0, -3)
		manifold.A.Set(1, 1, 8)
		So(manifold.Settle([]float64{-0.3, 0.4}, false), ShouldBeNil)
		So(manifold.Learn(nil), ShouldBeNil)

		var decomposition mat.SVD
		So(decomposition.Factorize(manifold.A, mat.SVDThin), ShouldBeTrue)
		operatorNorm := decomposition.Values(nil)[0]

		Convey("It should project the induced norm back inside contraction", func() {
			So(operatorNorm, ShouldBeLessThanOrEqualTo, manifold.cfg.TemporalNormMax+1e-12)
		})
	})
}

func TestResonanceManifoldSetAlpha(testingTB *testing.T) {
	Convey("Given an invalid replacement learning pace", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{1, 1}, 0, 0.05)
		So(err, ShouldBeNil)
		before := manifold.cfg

		Convey("It should reject the value without poisoning retained pace", func() {
			So(manifold.SetAlpha(math.NaN()), ShouldNotBeNil)
			So(manifold.SetAlpha(0), ShouldNotBeNil)
			So(manifold.SetAlpha(2), ShouldNotBeNil)
			So(manifold.cfg, ShouldResemble, before)
		})
	})
}

func TestResonanceManifoldDynamicHorizon(testingTB *testing.T) {
	Convey("Given relative precision without absolute baseline skill", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{1, 1}, 1, 0.05)
		So(err, ShouldBeNil)
		manifold.taskScaleReady[0] = true
		manifold.taskPrecision.Set(0, 0, 1)
		manifold.taskSkillReady[0] = true
		manifold.taskSkill.Set(0, 0, 0.5)

		_, retractedReach := manifold.DynamicHorizon(1, 5, 10)
		manifold.taskSkill.Set(0, 0, 2)
		_, grownReach := manifold.DynamicHorizon(1, 5, 10)

		Convey("It should retract a consistently unskilled head and grow a skilled one", func() {
			So(retractedReach, ShouldEqual, 4)
			So(grownReach, ShouldEqual, 6)
		})
	})

	Convey("Given certain confidence and a rollout that reaches zero retention", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{1, 1}, 1, 0.05)
		So(err, ShouldBeNil)
		manifold.taskScaleReady[0] = true
		manifold.taskPrecision.Set(0, 0, 1)
		manifold.taskSkillReady[0] = true
		manifold.taskSkill.Set(0, 0, 2)
		manifold.z[len(manifold.z)-1].Set(0, 0, 1)
		manifold.A.Zero()

		horizon, grownReach := manifold.DynamicHorizon(1, 5, 10)

		Convey("It should exclude the first unsupported rollout step", func() {
			So(horizon, ShouldEqual, 1)
			So(grownReach, ShouldEqual, 6)
		})
	})
}

func TestRolloutTaskPrediction(testingTB *testing.T) {
	Convey("Given a task head trained on the currently settled state", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{2, 4, 2}, 1, 0.03)
		So(err, ShouldBeNil)

		for sampleIndex := range 16 {
			input := []float64{
				math.Sin(float64(sampleIndex)),
				math.Cos(float64(sampleIndex)),
			}
			target := []float64{0.001 * float64(sampleIndex%3-1)}
			So(manifold.Settle(input, false), ShouldBeNil)
			So(manifold.Learn(target), ShouldBeNil)
		}

		So(manifold.Settle([]float64{0.4, -0.7}, false), ShouldBeNil)
		direct := manifold.TaskPrediction()
		curve := manifold.RolloutTaskPrediction(3)

		Convey("Its first rollout step should be the direct next-return prediction", func() {
			So(curve, ShouldHaveLength, 3)
			So(curve[0], ShouldAlmostEqual, direct[0])
		})
	})
}

func TestRolloutTaskForecast(testingTB *testing.T) {
	Convey("Given a task head with resolved prequential noise", testingTB, func() {
		manifold, err := NewResonanceManifold([]int{2, 4, 2}, 1, 0.03)
		So(err, ShouldBeNil)

		for sampleIndex := range 16 {
			input := []float64{
				math.Sin(float64(sampleIndex)),
				math.Cos(float64(sampleIndex)),
			}
			target := []float64{0.001 * math.Sin(float64(sampleIndex)*0.5)}
			So(manifold.Settle(input, false), ShouldBeNil)
			So(manifold.Learn(target), ShouldBeNil)
		}

		So(manifold.Settle([]float64{0.4, -0.7}, false), ShouldBeNil)
		direct := manifold.TaskPrediction()
		forecast, err := manifold.RolloutTaskForecast(3)

		Convey("It should align posterior uncertainty with the same first step", func() {
			So(err, ShouldBeNil)
			So(forecast, ShouldHaveLength, 3)
			So(forecast[0].Value, ShouldAlmostEqual, direct[0])
			So(forecast[0].Ready, ShouldBeTrue)
			So(forecast[0].Scale, ShouldBeGreaterThan, 0)
			So(forecast[0].DegreesOfFreedom, ShouldBeGreaterThan, 0)
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

func BenchmarkResonanceManifoldLearn(testingTB *testing.B) {
	manifold, err := NewResonanceManifold([]int{8, 16, 8}, 1, 0.01)

	if err != nil {
		testingTB.Fatal(err)
	}

	input := []float64{0.1, -0.2, 0.3, -0.4, 0.5, -0.6, 0.7, -0.8}

	if err := manifold.Settle(input, false); err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if err := manifold.Learn([]float64{0.01}); err != nil {
			testingTB.Fatal(err)
		}
	}
}

func BenchmarkRolloutTaskForecast(testingTB *testing.B) {
	manifold, err := NewResonanceManifold([]int{8, 16, 8}, 1, 0.01)

	if err != nil {
		testingTB.Fatal(err)
	}

	input := []float64{0.1, -0.2, 0.3, -0.4, 0.5, -0.6, 0.7, -0.8}

	for sampleIndex := range 8 {
		if err := manifold.Settle(input, false); err != nil {
			testingTB.Fatal(err)
		}

		if err := manifold.Learn([]float64{0.001 * float64(sampleIndex%3-1)}); err != nil {
			testingTB.Fatal(err)
		}
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := manifold.RolloutTaskForecast(4); err != nil {
			testingTB.Fatal(err)
		}
	}
}

func BenchmarkResonanceManifoldSettleSymm(testingTB *testing.B) {
	manifold, err := NewResonanceManifold([]int{5, 5, 5}, 1, 0.01)

	if err != nil {
		testingTB.Fatal(err)
	}

	input := []float64{0.31, -0.17, 0.23, -0.11, 0.07}
	target := []float64{0.01}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if _, err := manifold.SettleFromBatch(input, target); err != nil {
			testingTB.Fatal(err)
		}
	}
}

func BenchmarkResonanceManifoldSettleOnlySymm(testingTB *testing.B) {
	manifold, err := NewResonanceManifold([]int{5, 5, 5}, 1, 0.01)

	if err != nil {
		testingTB.Fatal(err)
	}

	manifold.SetStreamLearn(false)

	input := []float64{0.31, -0.17, 0.23, -0.11, 0.07}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		if err := manifold.Settle(input, true); err != nil {
			testingTB.Fatal(err)
		}
	}
}

func TestDenseHelpersMatchGonum(testingTB *testing.T) {
	Convey("Given column vectors and a weight matrix", testingTB, func() {
		left := mat.NewDense(4, 1, nil)
		right := mat.NewDense(8, 1, nil)
		signal := mat.NewDense(4, 1, nil)
		weight := mat.NewDense(4, 8, nil)

		for rowIndex := 0; rowIndex < 4; rowIndex++ {
			left.Set(rowIndex, 0, math.Sin(float64(rowIndex)*0.7))
			signal.Set(rowIndex, 0, math.Cos(float64(rowIndex)*0.3))

			for colIndex := 0; colIndex < 8; colIndex++ {
				weight.Set(rowIndex, colIndex, math.Tan(float64(rowIndex+colIndex)*0.11))
			}
		}

		for rowIndex := 0; rowIndex < 8; rowIndex++ {
			right.Set(rowIndex, 0, math.Sin(float64(rowIndex)*0.5+1))
		}

		gonumOuter := mat.NewDense(4, 8, nil)
		gonumOuter.Outer(1.0, left.ColView(0), right.ColView(0))

		denseOuter := mat.NewDense(4, 8, nil)
		denseOuterColsInto(denseOuter, left, right, 1.0)

		gonumTanh := mat.NewDense(4, 1, nil)
		gonumTanh.Apply(func(rowIndex, colIndex int, value float64) float64 {
			return math.Tanh(value)
		}, left)

		denseTanh := mat.DenseCopyOf(left)
		denseApplyTanhInPlace(denseTanh)

		gonumOneMinus := mat.NewDense(4, 1, nil)
		gonumOneMinus.Apply(func(rowIndex, colIndex int, value float64) float64 {
			return 1.0 - value*value
		}, left)

		denseOneMinus := mat.NewDense(4, 1, nil)
		denseApplyOneMinusSquareInto(denseOneMinus, left)

		gonumTranspose := mat.NewDense(8, 1, nil)
		gonumTranspose.Mul(weight.T(), signal)

		denseTranspose := mat.NewDense(8, 1, nil)
		denseMulWeightTransposeInto(denseTranspose, weight, signal)

		Convey("Dense helpers should match gonum element-wise", func() {
			So(mat.Equal(gonumOuter, denseOuter), ShouldBeTrue)
			So(mat.Equal(gonumTanh, denseTanh), ShouldBeTrue)
			So(mat.Equal(gonumOneMinus, denseOneMinus), ShouldBeTrue)
			So(mat.Equal(gonumTranspose, denseTranspose), ShouldBeTrue)
			So(denseColDot(left, left), ShouldAlmostEqual, mat.Dot(left.ColView(0), left.ColView(0)), 1e-15)
			So(denseColNorm(left), ShouldAlmostEqual, mat.Norm(left, 2), 1e-15)
		})
	})
}

func BenchmarkTaskPrediction(testingTB *testing.B) {
	manifold, err := NewResonanceManifold([]int{8, 16, 8}, 2, 0.01)

	if err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = manifold.TaskPrediction()
	}
}

func BenchmarkLatentState(testingTB *testing.B) {
	manifold, err := NewResonanceManifold([]int{8, 16, 8}, 2, 0.01)

	if err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = manifold.LatentState()
	}
}

func BenchmarkWireSnapshot(testingTB *testing.B) {
	manifold, err := NewResonanceManifold([]int{8, 16, 8}, 2, 0.01)

	if err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		manifold.WireSnapshot()
	}
}

func BenchmarkRolloutRetention(testingTB *testing.B) {
	manifold, err := NewResonanceManifold([]int{8, 16, 8}, 2, 0.01)

	if err != nil {
		testingTB.Fatal(err)
	}

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = manifold.RolloutRetention(16)
	}
}
