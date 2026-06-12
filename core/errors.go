package core

import "errors"

/*
ErrNilNumber is returned when a propagation chain contains a nil number.
*/
var ErrNilNumber = errors.New("nomagique: nil number")

/*
ErrEmptyInputs is returned when Observe receives no usable samples.
*/
var ErrEmptyInputs = errors.New("nomagique: empty inputs")

/*
ErrZeroPredicted is returned when a predicted value is zero.
*/
var ErrZeroPredicted = errors.New("nomagique: zero predicted value")

/*
ErrInvalidOutcome is returned when a Bernoulli outcome lies outside the unit interval.
*/
var ErrInvalidOutcome = errors.New("nomagique: invalid Bernoulli outcome")
