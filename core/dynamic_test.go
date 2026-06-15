package core

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

/*
pipelineStage is a non-scalar Number input for adversarial Observe tests.
*/
type pipelineStage[T ~float64] struct {
	result Scalar[T]
}

func (stage *pipelineStage[T]) Observe(...Number[T]) Scalar[T] {
	return stage.result
}

func (stage *pipelineStage[T]) Reset() error {
	return nil
}

/*
addStage adds a fixed delta to the carried scalar sample.
*/
type addStage[T ~float64] struct {
	delta float64
}

func (stage *addStage[T]) Observe(inputs ...Number[T]) Scalar[T] {
	sample, ok := inputs[0].(Scalar[T])

	if !ok {
		return Scalar[T](T(stage.delta))
	}

	return Scalar[T](T(float64(sample) + stage.delta))
}

func (stage *addStage[T]) Reset() error {
	return nil
}

/*
scaleStage multiplies the carried scalar sample by a fixed factor.
*/
type scaleStage[T ~float64] struct {
	factor float64
}

func (stage *scaleStage[T]) Observe(inputs ...Number[T]) Scalar[T] {
	sample, ok := inputs[0].(Scalar[T])

	if !ok {
		return 0
	}

	return Scalar[T](T(float64(sample) * stage.factor))
}

func (stage *scaleStage[T]) Reset() error {
	return nil
}

func TestScalar_Observe(testingTB *testing.T) {
	cases := []struct {
		name   string
		start  float64
		stages []Number[float64]
		expect float64
	}{
		{
			name:   "empty stages echo",
			start:  10,
			stages: nil,
			expect: 10,
		},
		{
			name:   "single add",
			start:  5,
			stages: []Number[float64]{&addStage[float64]{delta: 3}},
			expect: 8,
		},
		{
			name:  "add then scale",
			start: 5,
			stages: []Number[float64]{
				&addStage[float64]{delta: 3},
				&scaleStage[float64]{factor: 2},
			},
			expect: 16,
		},
		{
			name:  "scale then add",
			start: 5,
			stages: []Number[float64]{
				&scaleStage[float64]{factor: 2},
				&addStage[float64]{delta: 3},
			},
			expect: 13,
		},
		{
			name:   "negative sample",
			start:  -4,
			stages: []Number[float64]{&addStage[float64]{delta: 10}},
			expect: 6,
		},
		{
			name:   "zero start",
			start:  0,
			stages: []Number[float64]{&scaleStage[float64]{factor: 100}},
			expect: 0,
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			got := Scalar[float64](testCase.start).Observe(testCase.stages...)

			Convey("It should derive the expected scalar", func() {
				So(float64(got), ShouldEqual, testCase.expect)
			})
		})
	}

	Convey("Given a pipeline stage in the chain", testingTB, func() {
		got := Scalar[float64](5).Observe(
			&addStage[float64]{delta: 1},
			&pipelineStage[float64]{result: Scalar[float64](99)},
			&scaleStage[float64]{factor: 0.5},
		)

		Convey("It should apply stages in order", func() {
			So(float64(got), ShouldEqual, 49.5)
		})
	})
}

func TestScalar_Reset(testingTB *testing.T) {
	Convey("Given a scalar", testingTB, func() {
		scalar := Scalar[float64](10)

		Convey("When reset", func() {
			err := scalar.Reset()

			Convey("It should succeed without mutating the sample", func() {
				So(err, ShouldBeNil)
				So(float64(scalar), ShouldEqual, 10)
			})
		})
	})
}

func TestScalars_Observe(testingTB *testing.T) {
	cases := []struct {
		name   string
		start  []float64
		stages []Number[float64]
		expect []float64
	}{
		{
			name:   "empty stages echo",
			start:  []float64{1, 2, 3},
			stages: nil,
			expect: []float64{1, 2, 3},
		},
		{
			name:   "uniform add per element",
			start:  []float64{1, 2, 3},
			stages: []Number[float64]{&addStage[float64]{delta: 10}},
			expect: []float64{11, 12, 13},
		},
		{
			name:  "add then scale per element",
			start: []float64{0, 5, 10},
			stages: []Number[float64]{
				&addStage[float64]{delta: 2},
				&scaleStage[float64]{factor: 3},
			},
			expect: []float64{6, 21, 36},
		},
		{
			name:   "single element",
			start:  []float64{7},
			stages: []Number[float64]{&scaleStage[float64]{factor: 2}},
			expect: []float64{14},
		},
		{
			name:   "empty slice",
			start:  []float64{},
			stages: []Number[float64]{&addStage[float64]{delta: 1}},
			expect: []float64{},
		},
		{
			name:   "negative samples",
			start:  []float64{-2, -1, 0},
			stages: []Number[float64]{&addStage[float64]{delta: 5}},
			expect: []float64{3, 4, 5},
		},
	}

	for _, testCase := range cases {
		testCase := testCase

		Convey("Given "+testCase.name, testingTB, func() {
			scalars := make(Scalars[float64], len(testCase.start))

			for index, sample := range testCase.start {
				scalars[index] = Scalar[float64](sample)
			}

			got := scalars.Observe(testCase.stages...)

			Convey("It should derive each element independently", func() {
				So(len(got), ShouldEqual, len(testCase.expect))

				for index, expect := range testCase.expect {
					So(float64(got[index]), ShouldEqual, expect)
				}
			})
		})
	}

	Convey("Given repeated in-place observation", testingTB, func() {
		scalars := Scalars[float64]{Scalar[float64](1), Scalar[float64](2)}

		scalars.Observe(&addStage[float64]{delta: 1})
		scalars.Observe(&addStage[float64]{delta: 1})

		Convey("It should accumulate transforms on the receiver", func() {
			So(float64(scalars[0]), ShouldEqual, 3)
			So(float64(scalars[1]), ShouldEqual, 4)
		})
	})
}

func TestScalars_Reset(testingTB *testing.T) {
	Convey("Given scalars", testingTB, func() {
		scalars := Scalars[float64]{Scalar[float64](1), Scalar[float64](2)}

		Convey("When reset", func() {
			err := scalars.Reset()

			Convey("It should succeed without mutating samples", func() {
				So(err, ShouldBeNil)
				So(float64(scalars[0]), ShouldEqual, 1)
				So(float64(scalars[1]), ShouldEqual, 2)
			})
		})
	})
}

func BenchmarkScalar_Observe(b *testing.B) {
	stages := []Number[float64]{
		&addStage[float64]{delta: 1},
		&scaleStage[float64]{factor: 1.01},
		&addStage[float64]{delta: -0.5},
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = Scalar[float64](10).Observe(stages...)
	}
}

func BenchmarkScalars_Observe(b *testing.B) {
	stages := []Number[float64]{
		&addStage[float64]{delta: 1},
		&scaleStage[float64]{factor: 1.01},
	}
	scalars := Scalars[float64]{
		Scalar[float64](1),
		Scalar[float64](2),
		Scalar[float64](3),
		Scalar[float64](4),
	}

	b.ReportAllocs()

	for b.Loop() {
		_ = scalars.Observe(stages...)
	}
}
