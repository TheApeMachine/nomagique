package nomagique_test

import (
	"testing"
	"time"

	"github.com/theapemachine/nomagique/adaptive"
	"github.com/theapemachine/nomagique/algorithm"
	"github.com/theapemachine/nomagique/core"
	"github.com/theapemachine/nomagique/correlation"
	"github.com/theapemachine/nomagique/geometry"
	"github.com/theapemachine/nomagique/learning"
	"github.com/theapemachine/nomagique/logic"
	"github.com/theapemachine/nomagique/probability"
	"github.com/theapemachine/nomagique/statistic"
	"github.com/theapemachine/nomagique/vector"
)

func assignNumber(stage core.Number[float64]) {
	_ = stage
}

func TestNumberInterfaceCompile(testingTB *testing.T) {
	assignNumber(adaptive.NewEMA[float64]())
	assignNumber(adaptive.NewDelta[float64]())
	assignNumber(adaptive.NewZScore[float64]())
	assignNumber(adaptive.NewFracDiff[float64]())
	assignNumber(adaptive.NewMomentum[float64]())
	assignNumber(adaptive.NewCompression[float64]())
	assignNumber(adaptive.NewVariance[float64]())
	assignNumber(adaptive.NewRange[float64]())
	assignNumber(adaptive.NewAccumulator[float64]())
	assignNumber(adaptive.NewTimeElastic[float64](time.Hour, 1e-6))

	assignNumber(statistic.NewMean[float64](nil))
	assignNumber(statistic.NewMedian[float64](nil))
	assignNumber(statistic.NewSum[float64]())
	assignNumber(statistic.NewMin[float64]())
	assignNumber(statistic.NewMax[float64]())
	assignNumber(statistic.NewStdDev[float64](nil))
	assignNumber(statistic.NewEntropy[float64](0))
	assignNumber(statistic.NewMedianAbsolute[float64](nil))

	assignNumber(probability.CUSUM[float64]())
	assignNumber(probability.Bernoulli[float64]())
	assignNumber(probability.Rank[float64]())
	assignNumber(probability.TransitionSurprise[float64](5, 0.1))

	assignNumber(learning.Forecast[float64]())
	assignNumber(learning.Weight[float64]())
	assignNumber(learning.SampleRatio[float64]())

	rls, err := learning.NewRLS[float64](1, 1000)

	if err != nil {
		testingTB.Fatal(err)
	}

	assignNumber(rls)

	assignNumber(geometry.NewVelocity[float64]())
	assignNumber(geometry.NewCoupling[float64]())
	assignNumber(geometry.NewModePartition[float64](
		0.5,
		[]float64{1, 2},
		[]float64{1, 2},
		[]float64{1, 0, 0, 1},
	))

	assignNumber(geometry.NewProcrustes[float64](nil, nil))

	assignNumber(geometry.NewRotor[float64]())
	assignNumber(geometry.NewTranslator[float64]())

	var motor geometry.Multivector

	motor.FromRotation(0, 1, 0, 0)
	assignNumber(geometry.NewSandwich[float64](motor))

	assignNumber(correlation.NewPearson[float64](nil))
	assignNumber(correlation.NewCovariance[float64](nil))
	assignNumber(correlation.NewIntervalSeries[float64](8))
	assignNumber(correlation.NewWindowSet[float64](8))
	assignNumber(correlation.NewContagion[float64](
		nil,
		correlation.TierWindows{},
		correlation.ContagionConfig{},
	))

	assignNumber(algorithm.NewTrust[float64]())

	assignNumber(logic.NewAnd[float64]())
	assignNumber(logic.NewOr[float64]())
	assignNumber(logic.NewNot[float64]())
	assignNumber(logic.NewXor[float64]())
	assignNumber(logic.NewCompare[float64]())
	assignNumber(logic.NewSelect[float64]())
	assignNumber(logic.NewGate[float64]())
	assignNumber(logic.NewMux[float64](2))
	assignNumber(logic.NewFirstMatch[float64]())
	assignNumber(logic.NewLatch[float64]())
}

func TestNumberInterfaceVectorCompile(testingTB *testing.T) {
	extractor, err := vector.NewFeatureExtractor(1, func(inputs []float64) float64 {
		return inputs[0]
	})

	if err != nil {
		testingTB.Fatal(err)
	}

	assignNumber(extractor)

	inputSlot, err := vector.NewInputSlot[float64](extractor, 0)

	if err != nil {
		testingTB.Fatal(err)
	}

	featureNode, err := vector.NewFeatureNode[float64](extractor, 0)

	if err != nil {
		testingTB.Fatal(err)
	}

	assignNumber(inputSlot)
	assignNumber(featureNode)
}
