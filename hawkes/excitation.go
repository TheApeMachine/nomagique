package hawkes

import (
	"time"

	"github.com/theapemachine/nomagique/decay"
)

/*
ExcitationState tracks running Hawkes excitation sums while walking marked events.
*/
type ExcitationState struct {
	buySupport  float64
	sellSupport float64
	lastTime    time.Time
	haveLast    bool
}

/*
DecayTo advances excitation sums to eventTime under exponential decay.
*/
func (state *ExcitationState) DecayTo(eventTime time.Time, beta float64) {
	if !state.haveLast || !eventTime.After(state.lastTime) {
		return
	}

	decayFactor := decay.ExpNeg(beta, eventTime.Sub(state.lastTime).Seconds())
	state.buySupport *= decayFactor
	state.sellSupport *= decayFactor
	state.lastTime = eventTime
}

/*
LogLikelihoodSum accumulates log intensities across marked events.
*/
func (state *ExcitationState) LogLikelihoodSum(
	marked []MarkedEvent,
	muBuy, muSell, alphaBB, alphaBS, alphaSB, alphaSS, beta float64,
) (float64, bool) {
	if len(marked) == 0 {
		return 0, false
	}

	state.lastTime = marked[0].At
	state.haveLast = true
	logSum := 0.0

	for index := 0; index < len(marked); {
		eventTime := marked[index].At
		state.DecayTo(eventTime, beta)

		end := index

		for end < len(marked) && marked[end].At.Equal(eventTime) {
			end++
		}

		for _, event := range marked[index:end] {
			switch event.Side {
			case sideBuy:
				lambda := muBuy + alphaBB*state.buySupport + alphaBS*state.sellSupport

				if lambda <= 0 {
					return 0, false
				}

				logSum += decay.LogPositive(lambda)
			case sideSell:
				lambda := muSell + alphaSB*state.buySupport + alphaSS*state.sellSupport

				if lambda <= 0 {
					return 0, false
				}

				logSum += decay.LogPositive(lambda)
			}
		}

		for _, event := range marked[index:end] {
			switch event.Side {
			case sideBuy:
				state.buySupport += 1
			case sideSell:
				state.sellSupport += 1
			}
		}

		index = end
	}

	return logSum, true
}
