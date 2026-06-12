package core

import "errors"

type echoStage struct{}

func (echoStage) Observe(inputs ...Number) Float64 {
	if len(inputs) == 0 {
		return 0
	}

	sample, ok := inputs[len(inputs)-1].(Float64)

	if !ok {
		return 0
	}

	return sample
}

func (echoStage) Reset() error {
	return nil
}

type errStage struct {
	err error
}

func (stage errStage) Observe(inputs ...Number) Float64 {
	return 0
}

func (stage errStage) Apply(
	out Float64, work []Float64,
) (Float64, error) {
	return 0, stage.err
}

func (stage errStage) Reset() error {
	return nil
}

type notFloatNumber struct{}

func (notFloatNumber) Observe(inputs ...Number) Float64 {
	return 0
}

func (notFloatNumber) Reset() error {
	return nil
}

var errStageFailed = errors.New("stage failed")

type fastStage struct{}

func (fastStage) Observe(inputs ...Number) Float64 {
	return 0
}

func (fastStage) Reset() error {
	return nil
}

func (fastStage) Apply(
	out Float64, work []Float64,
) (Float64, error) {
	if len(work) == 0 {
		return out, nil
	}

	return work[len(work)-1], nil
}
