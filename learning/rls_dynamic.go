package learning

import (
	"github.com/theapemachine/nomagique/core"
)

/*
RLS is an online recursive-least-squares stage composable in nomagique.Number pipelines.

Observe ingests feature scalars followed by one target scalar. When fewer than
dimension+1 scalars are supplied, the prior output is returned.
*/
type RLS[T ~float64] struct {
	filter *RLSFilter
	output core.Scalar[T]
}

/*
NewRLS allocates an RLS stage with the given feature dimension excluding intercept.
*/
func NewRLS[T ~float64](dimension int, initialVariance float64) (*RLS[T], error) {
	filter, err := NewRLSFilter(dimension, initialVariance)

	if err != nil {
		return nil, err
	}

	return &RLS[T]{
		filter: filter,
	}, nil
}

/*
Observe updates coefficients from feature scalars and a trailing target scalar.
*/
func (rls *RLS[T]) Observe(inputs ...core.Number[T]) core.Scalar[T] {
	if rls == nil || rls.filter == nil {
		return core.Scalar[T](0)
	}

	scalars, ok := collectScalars[T](inputs...)

	if !ok || len(scalars) < rls.filter.dimension+1 {
		return rls.output
	}

	features := scalars[:rls.filter.dimension]
	target := scalars[len(scalars)-1]

	observeErr := rls.filter.Observe(features, target)

	if observeErr != nil {
		return rls.output
	}

	prediction, predictErr := rls.filter.Predict(features)

	if predictErr != nil {
		return rls.output
	}

	rls.output = core.Scalar[T](T(prediction))

	return rls.output
}

/*
Reset restores coefficients and covariance to their initial state.
*/
func (rls *RLS[T]) Reset() error {
	if rls == nil || rls.filter == nil {
		return nil
	}

	rls.filter.Reset()
	rls.output = core.Scalar[T](0)

	return nil
}

/*
Filter returns the underlying RLS filter for coefficient inspection.
*/
func (rls *RLS[T]) Filter() *RLSFilter {
	if rls == nil {
		return nil
	}

	return rls.filter
}
