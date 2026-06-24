package learning

import (
	"math"

	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
TrustWeight is a self-adapting rate from prediction error.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type TrustWeight struct {
	artifact *datura.Artifact
}

/*
NewTrustWeight returns a trust weight stage wired from config attributes on the artifact.
*/
func NewTrustWeight(artifact *datura.Artifact) *TrustWeight {
	return Weight(artifact)
}

/*
Weight returns a trust weight stage wired from config attributes on the artifact.
*/
func Weight(artifact *datura.Artifact) *TrustWeight {
	return &TrustWeight{
		artifact: artifact,
	}
}

func (trustWeight *TrustWeight) Read(payload []byte) (int, error) {
	state := datura.Acquire("trust-weight-state", datura.APPJSON)

	if _, err := state.Write(trustWeight.artifact.DecryptPayload()); err != nil {
		state.Release()

		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"trust-weight: state write failed",
			err,
		))
	}

	defer state.Release()

	predicted, actual, err := trustWeight.resolvePair(state)

	if err != nil {
		return 0, err
	}

	residual := actual - predicted
	trust := datura.Peek[float64](trustWeight.artifact, "output", "trust")
	prev := datura.Peek[float64](trustWeight.artifact, "output", "prev")
	minimum := datura.Peek[float64](trustWeight.artifact, "output", "min")
	maximum := datura.Peek[float64](trustWeight.artifact, "output", "max")
	rate := datura.Peek[float64](trustWeight.artifact, "output", "rate")
	count := int(datura.Peek[float64](trustWeight.artifact, "output", "count"))
	derived := trust

	if count == 0 {
		prev = predicted
		minimum = residual
		maximum = residual
		trust = 1
		count = 1
		derived = trust
	}

	if count > 1 {
		minimum = math.Min(minimum, residual)
		maximum = math.Max(maximum, residual)
		count++
	}

	if count == 1 && residual != minimum {
		minimum = math.Min(minimum, residual)
		maximum = math.Max(maximum, residual)
		count = 2
	}

	span := maximum - minimum

	if count > 1 {
		if span == 0 {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"trust-weight: residual span is zero",
				nil,
			))
		}

		surprise := absExact(residual) / span
		rate = surprise
		targetTrust := 1 - surprise

		if targetTrust < 0 {
			targetTrust = 0
		}

		trust += surprise * (targetTrust - trust)
		prev = predicted
		derived = trust
	}

	trustWeight.artifact.Poke(trust, "output", "trust")
	trustWeight.artifact.Poke(prev, "output", "prev")
	trustWeight.artifact.Poke(minimum, "output", "min")
	trustWeight.artifact.Poke(maximum, "output", "max")
	trustWeight.artifact.Poke(rate, "output", "rate")
	trustWeight.artifact.Poke(float64(count), "output", "count")
	trustWeight.artifact.Poke(derived, "output", "value")
	state.MergeOutput("value", derived)
	state.MergeOutput("predicted", predicted)
	state.MergeOutput("actual", actual)
	state.Poke("output", "root")
	state.Poke([]string{"value", "predicted", "actual"}, "inputs")
	return state.Read(payload)
}

func (trustWeight *TrustWeight) resolvePair(state *datura.Artifact) (float64, float64, error) {
	parsedPredicted, parsedActual, err := wirePair(trustWeight.artifact, state, "trust-weight")

	if err != nil {
		return 0, 0, err
	}

	if parsedActual == 0 {
		return 0, 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"trust-weight: actual must be non-zero",
			nil,
		))
	}

	return parsedPredicted, parsedActual, nil
}

func (trustWeight *TrustWeight) Write(payload []byte) (int, error) {
	trustWeight.artifact.WithPayload(payload)
	return len(payload), nil
}

func (trustWeight *TrustWeight) Close() error {
	return nil
}
