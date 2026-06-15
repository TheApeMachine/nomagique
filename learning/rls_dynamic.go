package learning

import (
	"github.com/theapemachine/datura"
)

/*
RLS is an online recursive-least-squares stage composable in nomagique.Number pipelines.

Read ingests feature scalars followed by one target scalar. When fewer than
dimension+1 scalars are supplied, the prior output is returned.
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
		artifact: datura.Acquire("rls", datura.Artifact_Type_json),
		filter:   filter,
	}, nil
}

func (rls *RLS) Write(p []byte) (int, error) {
	return rls.artifact.Write(p)
}

func (rls *RLS) Read(p []byte) (int, error) {
	if rls == nil || rls.filter == nil {
		return rls.artifact.Read(p)
	}

	values := float64Batch(rls.artifact)

	if len(values) >= rls.filter.dimension+1 {
		features := values[:rls.filter.dimension]
		target := values[len(values)-1]
		observeErr := rls.filter.Observe(features, target)

		if observeErr == nil {
			prediction, predictErr := rls.filter.Predict(features)

			if predictErr == nil {
				rls.output = prediction
				putFloat64Payload(&rls.artifact, "rls-dynamic", rls.output)
			}
		}
	}

	return rls.artifact.Read(p)
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
