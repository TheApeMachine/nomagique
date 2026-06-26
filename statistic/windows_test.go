package statistic

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

func writeWindowsWire(stage *Windows, wire *datura.Artifact) error {
	packed, err := wire.Message().MarshalPacked()

	if err != nil {
		return err
	}

	_, err = stage.Write(packed)

	return err
}

func TestWindowsRead(testingTB *testing.T) {
	Convey("Given explicit window hints", testingTB, func() {
		config := datura.Acquire("windows-test-config", datura.APPJSON)
		config.Poke(5.0, "config", "shortHint")
		config.Poke(60.0, "config", "longHint")
		stage := NewWindows(config)
		wire := datura.Acquire("windows-test-wire", datura.APPJSON)
		wire.Poke([]float64{1, 2, 3}, "history")

		err := writeWindowsWire(stage, wire)
		wire.Release()
		config.Release()

		So(err, ShouldBeNil)

		buffer := make([]byte, 65536)
		readCount, err := stage.Read(buffer)

		Convey("It should return the configured hints", func() {
			if err != nil && err != io.EOF {
				So(err, ShouldBeNil)
			}
			So(err, ShouldNotEqual, io.ErrShortBuffer)

			out := datura.Acquire("windows-test-out", datura.APPJSON)
			_, writeErr := out.Unpack(buffer[:readCount])
			So(writeErr, ShouldBeNil)
			So(datura.Peek[float64](out, "output", "shortWindow"), ShouldEqual, 5)
			So(datura.Peek[float64](out, "output", "longWindow"), ShouldEqual, 60)
			out.Release()
		})
	})

	Convey("Given history without hints", testingTB, func() {
		history := []float64{1, 1, 1, 1, 10, 12, 9, 11}
		config := datura.Acquire("windows-test-config", datura.APPJSON)
		stage := NewWindows(config)
		wire := datura.Acquire("windows-test-wire", datura.APPJSON)
		wire.Poke(history, "history")

		err := writeWindowsWire(stage, wire)
		wire.Release()
		config.Release()

		So(err, ShouldBeNil)

		buffer := make([]byte, 65536)
		readCount, err := stage.Read(buffer)

		Convey("It should derive short and long windows from the sample spread", func() {
			if err != nil && err != io.EOF {
				So(err, ShouldBeNil)
			}

			out := datura.Acquire("windows-test-out", datura.APPJSON)
			_, writeErr := out.Unpack(buffer[:readCount])
			So(writeErr, ShouldBeNil)
			So(datura.Peek[float64](out, "output", "shortWindow"), ShouldEqual, 3)
			So(datura.Peek[float64](out, "output", "longWindow"), ShouldEqual, float64(len(history)))
			out.Release()
		})
	})

	Convey("Given empty history without hints", testingTB, func() {
		config := datura.Acquire("windows-test-config", datura.APPJSON)
		stage := NewWindows(config)
		wire := datura.Acquire("windows-test-wire", datura.APPJSON)

		err := writeWindowsWire(stage, wire)
		wire.Release()
		config.Release()

		So(err, ShouldBeNil)

		buffer := make([]byte, 65536)
		_, err = stage.Read(buffer)

		Convey("It should reject missing history", func() {
			So(err, ShouldNotBeNil)
		})
	})
}

func BenchmarkWindowsRead(testingTB *testing.B) {
	history := make([]float64, 128)

	for index := range history {
		history[index] = float64(index + 1)
	}

	config := datura.Acquire("windows-bench-config", datura.APPJSON)
	stage := NewWindows(config)
	wire := datura.Acquire("windows-bench-wire", datura.APPJSON)
	wire.Poke(history, "history")
	_ = writeWindowsWire(stage, wire)
	wire.Release()
	buffer := make([]byte, 65536)

	testingTB.ReportAllocs()

	for testingTB.Loop() {
		_, _ = stage.Read(buffer)
	}

	config.Release()
}
