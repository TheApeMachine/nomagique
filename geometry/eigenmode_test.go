package geometry

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

func TestEigenmodeMembers(t *testing.T) {
	t.Parallel()

	Convey("Members returns an independent copy of the mode list", t, func() {
		mode := &Eigenmode{
			members: []uint64{1, 2, 3},
			energy:  4.5,
		}

		copySlice := mode.Members()

		copySlice[0] = 99

		So(mode.members[0], ShouldEqual, 1)
		So(len(copySlice), ShouldEqual, 3)
	})
}

func TestEigenmodeEnergy(t *testing.T) {
	t.Parallel()

	Convey("Energy exposes the aggregate score", t, func() {
		mode := &Eigenmode{energy: 12.25}

		So(mode.Energy(), ShouldEqual, 12.25)
	})
}

func TestModePartition_Observe(t *testing.T) {
	t.Parallel()

	Convey("Given no participants", t, func() {
		partition := NewModePartition(0.5, nil, nil, nil)
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, partition)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
		So(partition.Snap(), ShouldBeNil)
	})

	Convey("Given participants with pairwise coupling at threshold", t, func() {
		partition := NewModePartition(
			1.0,
			[]float64{10, 20, 30},
			[]float64{1, 2, 4},
			[]float64{
				1, 1, 0,
				1, 1, 0,
				0, 0, 1,
			},
		)
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, partition)

		So(err, ShouldBeNil)

		energy := datura.Peek[float64](artifact, "output", "value")

		So(energy, ShouldEqual, 4)
		So(len(partition.Snap().Modes()), ShouldEqual, 2)
		So(len(partition.Snap().Modes()[0].Members()), ShouldEqual, 2)
	})

	Convey("Given coupled participants", t, func() {
		partition := NewModePartition(
			1,
			[]float64{10, 20, 30},
			[]float64{1, 2, 4},
			[]float64{
				1, 1, 0,
				1, 1, 0,
				0, 0, 1,
			},
		)
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, partition)

		So(err, ShouldBeNil)

		energy := datura.Peek[float64](artifact, "output", "value")

		Convey("It should return dominant mode energy", func() {
			So(energy, ShouldEqual, 4)
			So(len(partition.Snap().Modes()), ShouldEqual, 2)
		})
	})

	Convey("Given mismatched stream lengths", t, func() {
		partition := NewModePartition(
			1,
			[]float64{10, 20},
			[]float64{1},
			[]float64{1, 0, 0, 1},
		)
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, partition)

		So(err, ShouldBeNil)

		Convey("It should return zero output", func() {
			So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 0)
		})
	})
}

func TestModePartition_Reset(t *testing.T) {
	Convey("Given an observed mode partition", t, func() {
		partition := NewModePartition(
			1,
			[]float64{10, 20, 30},
			[]float64{1, 2, 4},
			[]float64{
				1, 1, 0,
				1, 1, 0,
				0, 0, 1,
			},
		)
		artifact := datura.Acquire("test", datura.APPJSON)
		err := transport.NewFlipFlop(artifact, partition)

		So(err, ShouldBeNil)
		So(partition.Reset(), ShouldBeNil)

		Convey("It should clear snap", func() {
			So(partition.Snap(), ShouldBeNil)
		})
	})
}

func BenchmarkModePartition_Observe(testingTB *testing.B) {
	size := 16
	origins := make([]float64, size)
	energies := make([]float64, size)
	matrix := make([]float64, size*size)

	for index := range origins {
		origins[index] = float64(index + 1)
		energies[index] = float64(index%5 + 1)

		for col := range size {
			value := 0.0

			if (index+col)%3 == 0 {
				value = 1
			}

			matrix[index*size+col] = value
		}
	}

	partition := NewModePartition(0.9, origins, energies, matrix)
	artifact := datura.Acquire("test", datura.APPJSON)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_ = transport.NewFlipFlop(artifact, partition)
	}
}
