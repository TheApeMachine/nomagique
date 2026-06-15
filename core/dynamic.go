package core

/*
Number is a reactive numeric primitive. Observe applies pipeline stages to the
carried sample; Reset clears derived state.
*/
type Number[T ~float64] interface {
	Observe(...Number[T]) Scalar[T]
	Reset() error
}

/*
Scalar is a reactive numeric primitive. Observe applies pipeline stages to the
carried sample; Reset clears derived state.
*/
type Scalar[T ~float64] float64

/*
Observe applies the given numbers to the receiver.
*/
func (scalar Scalar[T]) Observe(stages ...Number[T]) Scalar[T] {
	for _, stage := range stages {
		scalar = stage.Observe(scalar)
	}

	return scalar
}

func (scalar Scalar[T]) Reset() error {
	return nil
}

/*
Scalars is a slice of scalars.
*/
type Scalars[T ~float64] []Scalar[T]

/*
Observe applies the given scalars to the receiver.
*/
func (scalars Scalars[T]) Observe(stages ...Number[T]) Scalars[T] {
	for index, scalar := range scalars {
		scalars[index] = scalar.Observe(stages...)
	}

	return scalars
}

/*
Reset clears derived state.
*/
func (scalars Scalars[T]) Reset() error {
	return nil
}
