package hawkes

import (
	"math"

	"gonum.org/v1/gonum/stat"
)

/*
MethodOfMoments derives a stable seed from empirical buy and sell count streams.
*/
func MethodOfMoments(
	buy, sell, weights []float64, beta float64,
) (BivariateParams, bool) {
	if beta <= 0 || len(buy) != len(sell) || len(buy) < 2 {
		return BivariateParams{}, false
	}

	meanBuy := stat.Mean(buy, weights)
	meanSell := stat.Mean(sell, weights)

	if meanBuy <= 0 || meanSell <= 0 {
		return BivariateParams{}, false
	}

	varianceBuy := stat.BivariateMoment(2, 0, buy, sell, weights)
	varianceSell := stat.BivariateMoment(0, 2, buy, sell, weights)
	covariance := stat.BivariateMoment(1, 1, buy, sell, weights)

	params := BivariateParams{Beta: beta}

	if varianceBuy > meanBuy {
		params.AlphaBB = (varianceBuy - meanBuy) * beta / (2 * meanBuy)
	}

	if varianceSell > meanSell {
		params.AlphaSS = (varianceSell - meanSell) * beta / (2 * meanSell)
	}

	if covariance > 0 {
		params.AlphaBS = covariance * beta / (2 * meanSell)
		params.AlphaSB = covariance * beta / (2 * meanBuy)
	}

	params.MuBuy = meanBuy * (1 - params.AlphaBB/beta)
	params.MuSell = meanSell * (1 - params.AlphaSS/beta)

	if params.MuBuy <= 0 || params.MuSell <= 0 || !params.Stable() {
		return BivariateParams{}, false
	}

	return params, true
}

/*
TheoreticalCentralMoment estimates the central mixed moment from fitted parameters.
*/
func TheoreticalCentralMoment(
	params BivariateParams, orderR, orderS float64,
) (float64, bool) {
	lambdaBuy, lambdaSell, ok := params.MeanIntensity()

	if !ok {
		return 0, false
	}

	branching := params.branchingMatrix()

	switch {
	case orderR == 2 && orderS == 0:
		return lambdaBuy + 2*branching[0][0]*lambdaBuy, true
	case orderR == 0 && orderS == 2:
		return lambdaSell + 2*branching[1][1]*lambdaSell, true
	case orderR == 1 && orderS == 1:
		return branching[0][1]*lambdaSell + branching[1][0]*lambdaBuy, true
	default:
		return 0, false
	}
}

/*
MomentConfidence returns a fit score in (0, 1] from empirical and theoretical moments.
*/
func MomentConfidence(
	empirical, theoretical float64,
) float64 {
	scale := math.Max(math.Abs(theoretical), math.Abs(empirical))

	if scale <= 0 {
		return 1
	}

	residual := math.Abs(empirical-theoretical) / scale

	return 1 / (1 + residual)
}

/*
CrossAsymmetry compares third-order mixed moments to expose leader/follower asymmetry.
*/
func CrossAsymmetry(buy, sell, weights []float64) (float64, bool) {
	if len(buy) != len(sell) || len(buy) < 2 {
		return 0, false
	}

	leadBuy := stat.BivariateMoment(2, 1, buy, sell, weights)
	leadSell := stat.BivariateMoment(1, 2, buy, sell, weights)

	return leadBuy - leadSell, true
}
