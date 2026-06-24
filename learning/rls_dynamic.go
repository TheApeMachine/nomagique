package learning

import (
	"github.com/theapemachine/datura"
	"github.com/theapemachine/errnie"
)

/*
RLS is an online recursive-least-squares stage composable in nomagique.Number pipelines.
The constructor artifact holds config; Write buffers inbound wire on its payload.
*/
type RLS struct {
	artifact *datura.Artifact
	filter   *RLSFilter
	output   float64
}

/*
NewRLS allocates an RLS stage with the given feature dimension excluding intercept.
*/
func NewRLS(dimension int, initialVariance float64) (*RLS, error) {
	filter, err := NewRLSFilter(dimension, initialVariance)

	if err != nil {
		return nil, err
	}

	return &RLS{
		artifact: datura.Acquire("rls-config", datura.APPJSON),
		filter:   filter,
	}, nil
}

func (rls *RLS) Read(payload []byte) (int, error) {
	if rls == nil || rls.filter == nil {
		return 0, nil
	}

	state := datura.Acquire("rls-state", datura.APPJSON)
	state.Inspect("learning", "rls", "Read()", "p")

	if _, err := state.Write(rls.artifact.DecryptPayload()); err != nil {
		return 0, err
	}

	values := datura.Peek[[]float64](state, "batch")

	if len(values) == 0 {
		values = datura.Peek[[]float64](state, "features")
	}

	if len(values) >= rls.filter.dimension+1 {
		features := values[:rls.filter.dimension]
		target := values[len(values)-1]

		if observeErr := rls.filter.Observe(features, target); observeErr != nil {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"rls: observe failed",
				observeErr,
			))
		}

		prediction, predictErr := rls.filter.Predict(features)

		if predictErr != nil {
			return 0, errnie.Error(errnie.Err(
				errnie.Validation,
				"rls: predict failed",
				predictErr,
			))
		}

		rls.output = prediction
		rls.artifact.Poke(rls.output, "output", "value")
	}

	value := rls.output

	if len(values) >= rls.filter.dimension+1 {
		value = datura.Peek[float64](rls.artifact, "output", "value")
	}

	if len(values) < rls.filter.dimension+1 {
		return 0, errnie.Error(errnie.Err(
			errnie.Validation,
			"rls: batch shorter than feature dimension plus target",
			nil,
		))
	}

	state.MergeOutput("value", value)
	state.Poke("output", "root")
	state.Poke([]string{"value"}, "inputs")
	return state.Read(payload)
}

func (rls *RLS) Write(payload []byte) (int, error) {
	rls.artifact.WithPayload(payload)
	return len(payload), nil
}

func (rls *RLS) Close() error {
	return nil
}

/*
Reset restores coefficients and covariance to their initial state.
*/
func (rls *RLS) Reset() error {
	if rls == nil || rls.filter == nil {
		return nil
	}

	rls.filter.Reset()
	rls.output = 0
	rls.artifact.WithAttributes(datura.Map[any]{})

	return nil
}

/*
Filter returns the underlying RLS filter for coefficient inspection.
*/
func (rls *RLS) Filter() *RLSFilter {
	if rls == nil {
		return nil
	}

	return rls.filter
}
