package hawkes

import (
	"math"
	"time"

	"github.com/theapemachine/nomagique/decay"
	"github.com/theapemachine/nomagique/timeline"
)

type likelihoodGradient struct {
	muX     float64
	muY     float64
	alphaXX float64
	alphaXY float64
	alphaYX float64
	alphaYY float64
	beta    float64
	logSum  float64
	valid   bool
}

/*
LogLikelihoodGradient returns log-likelihood and partial derivatives at horizon.
*/
func (fit BivariateFit) LogLikelihoodGradient(
	stream ArrivalStream,
	horizon time.Time,
) (logLikelihood float64, gradient [7]float64, ok bool) {
	if fit.MuX <= 0 || fit.MuY <= 0 || fit.Beta <= 0 {
		return math.Inf(-1), gradient, false
	}

	marked := stream.Marked()

	if len(marked) == 0 {
		return math.Inf(-1), gradient, false
	}

	span := stream.Span(horizon)

	if span <= 0 {
		return math.Inf(-1), gradient, false
	}

	eventGradient := fit.eventLogLikelihoodGradient(marked, fit.Beta)

	if !eventGradient.valid {
		return math.Inf(-1), gradient, false
	}

	buySupport, sellSupport := stream.kernelIntegralSupport(horizon, fit.Beta)
	buySupportBeta := kernelIntegralSupportBetaDerivative(stream.buy, horizon, fit.Beta)
	sellSupportBeta := kernelIntegralSupportBetaDerivative(stream.sell, horizon, fit.Beta)
	beta := fit.Beta

	compensator := fit.MuX*span +
		(fit.AlphaXX/beta)*buySupport +
		(fit.AlphaXY/beta)*sellSupport +
		fit.MuY*span +
		(fit.AlphaYX/beta)*buySupport +
		(fit.AlphaYY/beta)*sellSupport

	gradient[0] = eventGradient.muX - span
	gradient[1] = eventGradient.muY - span
	gradient[2] = eventGradient.alphaXX - buySupport/beta
	gradient[3] = eventGradient.alphaXY - sellSupport/beta
	gradient[4] = eventGradient.alphaYX - buySupport/beta
	gradient[5] = eventGradient.alphaYY - sellSupport/beta
	gradient[6] = eventGradient.beta - fit.compensatorBetaDerivative(
		buySupport, sellSupport, buySupportBeta, sellSupportBeta,
	)

	logLikelihood = eventGradient.logSum - compensator

	return logLikelihood, gradient, true
}

func (fit BivariateFit) eventLogLikelihoodGradient(
	marked []MarkedEvent,
	beta float64,
) likelihoodGradient {
	result := likelihoodGradient{valid: true}
	buyToBuy := 0.0
	sellToBuy := 0.0
	buyToSell := 0.0
	sellToSell := 0.0
	dBuyToBuy := 0.0
	dSellToBuy := 0.0
	dBuyToSell := 0.0
	dSellToSell := 0.0
	lastTime := marked[0].At
	haveLast := true

	for index := 0; index < len(marked); {
		eventTime := marked[index].At

		if haveLast && eventTime.After(lastTime) {
			decayFactor := decay.ExpNeg(beta, eventTime.Sub(lastTime).Seconds())
			age := eventTime.Sub(lastTime).Seconds()
			dBuyToBuy = (dBuyToBuy - buyToBuy*age) * decayFactor
			dSellToBuy = (dSellToBuy - sellToBuy*age) * decayFactor
			dBuyToSell = (dBuyToSell - buyToSell*age) * decayFactor
			dSellToSell = (dSellToSell - sellToSell*age) * decayFactor
			buyToBuy *= decayFactor
			sellToBuy *= decayFactor
			buyToSell *= decayFactor
			sellToSell *= decayFactor
			lastTime = eventTime
		}

		end := index

		for end < len(marked) && marked[end].At.Equal(eventTime) {
			end++
		}

		for _, event := range marked[index:end] {
			switch event.Side {
			case sideBuy:
				lambda := fit.MuX + fit.AlphaXX*buyToBuy + fit.AlphaXY*sellToBuy

				if lambda <= 0 {
					return likelihoodGradient{}
				}

				inverse := 1 / lambda
				lambdaBeta := fit.AlphaXX*dBuyToBuy + fit.AlphaXY*dSellToBuy
				result.logSum += math.Log(lambda)
				result.muX += inverse
				result.alphaXX += inverse * buyToBuy
				result.alphaXY += inverse * sellToBuy
				result.beta += inverse * lambdaBeta
			case sideSell:
				lambda := fit.MuY + fit.AlphaYX*buyToSell + fit.AlphaYY*sellToSell

				if lambda <= 0 {
					return likelihoodGradient{}
				}

				inverse := 1 / lambda
				lambdaBeta := fit.AlphaYX*dBuyToSell + fit.AlphaYY*dSellToSell
				result.logSum += math.Log(lambda)
				result.muY += inverse
				result.alphaYX += inverse * buyToSell
				result.alphaYY += inverse * sellToSell
				result.beta += inverse * lambdaBeta
			}
		}

		for _, event := range marked[index:end] {
			switch event.Side {
			case sideBuy:
				buyToBuy += 1
				buyToSell += 1
			case sideSell:
				sellToBuy += 1
				sellToSell += 1
			}
		}

		index = end
	}

	return result
}

func kernelIntegralSupportBetaDerivative(
	events timeline.Timeline,
	horizon time.Time,
	beta float64,
) float64 {
	derivative := 0.0

	for _, eventTime := range events.Times() {
		remaining := horizon.Sub(eventTime).Seconds()

		if remaining > 0 {
			derivative += remaining * decay.ExpNeg(beta, remaining)
		}
	}

	return derivative
}

func (fit BivariateFit) compensatorBetaDerivative(
	buySupport, sellSupport, buySupportBeta, sellSupportBeta float64,
) float64 {
	beta := fit.Beta
	branchX := fit.AlphaXX / beta
	branchCrossToX := fit.AlphaXY / beta
	branchCrossToY := fit.AlphaYX / beta
	branchY := fit.AlphaYY / beta

	return -branchX/beta*buySupport +
		branchX*buySupportBeta +
		-branchCrossToX/beta*sellSupport +
		branchCrossToX*sellSupportBeta +
		-branchCrossToY/beta*buySupport +
		branchCrossToY*buySupportBeta +
		-branchY/beta*sellSupport +
		branchY*sellSupportBeta
}

func logSpaceGradient(naturalGradient [7]float64, fit BivariateFit) [7]float64 {
	alphaContribution := naturalGradient[2]*fit.AlphaXX +
		naturalGradient[3]*fit.AlphaXY +
		naturalGradient[4]*fit.AlphaYX +
		naturalGradient[5]*fit.AlphaYY

	return [7]float64{
		naturalGradient[0] * fit.MuX,
		naturalGradient[1] * fit.MuY,
		naturalGradient[6]*fit.Beta + alphaContribution,
		naturalGradient[2] * fit.AlphaXX,
		naturalGradient[3] * fit.AlphaXY,
		naturalGradient[4] * fit.AlphaYX,
		naturalGradient[5] * fit.AlphaYY,
	}
}
