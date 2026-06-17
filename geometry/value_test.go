package geometry

import (
	"io"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestNewValueFromBytes(testingTB *testing.T) {
	Convey("Given empty payload", testingTB, func() {
		tokens, err := NewValueFromBytes(nil)

		Convey("It should reject empty input", func() {
			So(err, ShouldEqual, io.ErrShortBuffer)
			So(tokens, ShouldBeNil)
		})
	})

	Convey("Given a short byte payload", testingTB, func() {
		payload := []byte{0x01, 0x02, 0x03}
		tokens, err := NewValueFromBytes(payload)

		Convey("It should pack bytes into the first words", func() {
			So(err, ShouldBeNil)
			So(len(tokens), ShouldEqual, 1)
			So(tokens[0][0], ShouldEqual, uint64(0x030201))
		})
	})

	Convey("Given a payload spanning two words", testingTB, func() {
		payload := make([]byte, 10)

		for index := range payload {
			payload[index] = byte(index + 1)
		}

		tokens, err := NewValueFromBytes(payload)

		Convey("It should fill consecutive word lanes", func() {
			So(err, ShouldBeNil)
			So(len(tokens), ShouldEqual, 1)
			So(tokens[0][0], ShouldNotEqual, 0)
			So(tokens[0][1], ShouldNotEqual, 0)
		})
	})

	Convey("Given a payload beyond eight words", testingTB, func() {
		payload := make([]byte, 80)

		for index := range payload {
			payload[index] = 0xFF
		}

		tokens, err := NewValueFromBytes(payload)

		Convey("It should truncate at the eighth word", func() {
			So(err, ShouldBeNil)
			So(tokens[0][7], ShouldNotEqual, 0)
			So(tokens[0][8], ShouldEqual, 0)
		})
	})
}
