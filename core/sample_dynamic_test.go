package core

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type sampleEcho struct {
	last float64
}

func (echo *sampleEcho) Observe(inputs ...Number) Float64 {
	if len(inputs) == 0 {
		return 0
	}

	sample, ok := inputs[len(inputs)-1].(Float64)

	if !ok {
		return 0
	}

	return sample
}

func (echo *sampleEcho) ObserveSample(sample float64) float64 {
	echo.last = sample

	return sample * 2
}

func (echo *sampleEcho) Reset() error {
	echo.last = 0

	return nil
}

func TestSampleDynamic_contract(testingTB *testing.T) {
	Convey("Given a type satisfying SampleDynamic", testingTB, func() {
		echo := &sampleEcho{}
		var dynamic SampleDynamic = echo

		Convey("It should route ObserveSample separately from Observe", func() {
			So(dynamic.ObserveSample(4), ShouldEqual, 8)
			So(echo.last, ShouldEqual, 4)
			So(dynamic.Reset(), ShouldBeNil)
		})
	})
}
