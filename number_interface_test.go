package nomagique_test

import (
	"io"
	"testing"
	"time"

	"github.com/theapemachine/datura/transport"
	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/algorithm"
	"github.com/theapemachine/nomagique/correlation"
	"github.com/theapemachine/nomagique/geometry"
	"github.com/theapemachine/nomagique/learning"
	"github.com/theapemachine/nomagique/logic"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/statistic"
	"github.com/theapemachine/nomagique/vector"
)

func assignStage(stage io.ReadWriter) {
	_ = stage
}

func TestNumberInterfaceCompile(testingTB *testing.T) {
	assignStage(adaptive.NewEMA())
	assignStage(adaptive.NewDelta())
	assignStage(adaptive.NewZScore())
	assignStage(adaptive.NewFracDiff())
	assignStage(adaptive.NewMomentum())
	assignStage(adaptive.NewCompression())
	assignStage(adaptive.NewVariance())
	assignStage(adaptive.NewRange())
	assignStage(adaptive.NewAccumulator())
	assignStage(adaptive.NewTimeElastic(time.Hour, 1e-6))

	assignStage(statistic.NewMean(nil))
	assignStage(statistic.NewMedian(nil, nil))
	assignStage(statistic.NewSum())
	assignStage(statistic.NewMin())
	assignStage(statistic.NewMax())
	assignStage(statistic.NewStdDev(nil))
	assignStage(statistic.NewEntropy(0))
	assignStage(statistic.NewMedianAbsolute(nil))

	assignStage(probability.NewCUSUM())
	assignStage(probability.NewBernoulli())
	assignStage(probability.NewRank())
	assignStage(probability.NewTransitionSurprise(5, 0.1))

	classifier := probability.NewClassifier(
		logic.NewConstant(0.1),
		logic.NewConstant(0.2),
	)

	if classifier == nil {
		testingTB.Fatal("classifier required")
	}

	assignStage(classifier)

	assignStage(learning.Forecast())
	assignStage(learning.Weight())
	assignStage(learning.SampleRatio())

	rls, err := learning.NewRLS(1, 1000)

	if err != nil {
		testingTB.Fatal(err)
	}

	assignStage(rls)

	assignStage(geometry.NewVelocity())
	assignStage(geometry.NewCoupling())
	assignStage(geometry.NewModePartition(
		0.5,
		[]float64{1, 2},
		[]float64{1, 2},
		[]float64{1, 0, 0, 1},
	))

	assignStage(geometry.NewProcrustes(nil, nil))

	assignStage(geometry.NewRotor())
	assignStage(geometry.NewTranslator())

	var motor geometry.Multivector

	motor.FromRotation(0, 1, 0, 0)
	assignStage(geometry.NewSandwich(motor))

	assignStage(correlation.NewPearson(nil))
	assignStage(correlation.NewCovariance(nil))
	assignStage(correlation.NewIntervalSeries(8))
	assignStage(correlation.NewWindowSet(8))
	assignStage(correlation.NewContagion(
		nil,
		correlation.TierWindows{},
		correlation.ContagionConfig{},
	))

	assignStage(algorithm.NewTrust())
	assignStage(transport.NewThrough(0))

	assignStage(logic.NewCircuit(logic.Rules{
		{
			Condition: logic.True{Operand: true},
			Then:      logic.NewConstant(1),
		},
	}))
}

func TestNumberInterfaceVectorCompile(testingTB *testing.T) {
	extractor, err := vector.NewFeatureExtractor(1, func(inputs []float64) float64 {
		return inputs[0]
	})

	if err != nil {
		testingTB.Fatal(err)
	}

	assignStage(extractor)

	inputSlot := vector.NewInputSlot(extractor, 0)
	featureNode := vector.NewFeatureNode(extractor, 0)

	assignStage(inputSlot)
	assignStage(featureNode)
}
