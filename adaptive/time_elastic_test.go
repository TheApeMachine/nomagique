package adaptive

import (
	"io"
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/datura/transport"
)

var timeElasticConfig = datura.Acquire("time-elastic-config", datura.APPJSON).
	Poke(float64(time.Hour), "config", "halflife").
	Poke(1e-6, "config", "epsilon")

func TestTimeElasticRead(t *testing.T) {
	Convey("Given a TimeElastic", t, func() {
		timeElastic := NewTimeElastic(timeElasticConfig)
		input := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(float64(time.Unix(0, 1).UnixNano()), "at")
		io.Copy(timeElastic, input)

		Convey("When Read is called on the first sample", func() {
			frame := make([]byte, 65536)
			readCount, err := timeElastic.Read(frame)
			So(err, ShouldEqual, io.EOF)

			outbound := datura.Acquire("test-out", datura.APPJSON)
			_, err = outbound.Write(frame[:readCount])
			So(err, ShouldBeNil)

			rootKey := datura.Peek[string](outbound, "root")

			So(rootKey, ShouldEqual, "output")
			So(datura.Peek[float64](outbound, rootKey, "value"), ShouldEqual, 1)
		})

		Convey("When a later timestamp regresses", func() {
			_, _ = io.Copy(timeElastic, datura.Acquire("test", datura.APPJSON).
				Poke(10, "sample").
				Poke(float64(time.Unix(0, 2).UnixNano()), "at"))
			_, _ = timeElastic.Read(make([]byte, 65536))
			_, _ = io.Copy(timeElastic, datura.Acquire("test", datura.APPJSON).
				Poke(12, "sample").
				Poke(float64(time.Unix(0, 1).UnixNano()), "at"))

			_, err := timeElastic.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given missing halflife or epsilon", t, func() {
		timeElastic := NewTimeElastic(datura.Acquire("time-elastic-config-missing", datura.APPJSON))
		input := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(float64(time.Unix(0, 1).UnixNano()), "at")
		_, _ = io.Copy(timeElastic, input)

		Convey("When Read is called", func() {
			_, err := timeElastic.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a non-finite sample", t, func() {
		timeElastic := NewTimeElastic(timeElasticConfig)
		invalid := datura.Acquire("test", datura.APPJSON).
			Poke(math.NaN(), "sample").
			Poke(float64(time.Unix(0, 1).UnixNano()), "at")
		_, _ = io.Copy(timeElastic, invalid)

		Convey("When Read is called", func() {
			_, err := timeElastic.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})
	})
}

func TestTimeElasticWrite(t *testing.T) {
	Convey("Given a TimeElastic", t, func() {
		timeElastic := NewTimeElastic(datura.Acquire("time-elastic-config", datura.APPJSON).
			Poke(float64(time.Hour), "config", "halflife").
			Poke(1e-6, "config", "epsilon"))

		Convey("When Write is called", func() {
			input := datura.Acquire("test", datura.APPJSON).
				Poke(10, "sample").
				Poke(float64(time.Unix(0, 1).UnixNano()), "at")
			_, err := io.Copy(timeElastic, input)
			So(err, ShouldBeNil)
		})
	})
}

func TestTimeElasticFlipFlop(t *testing.T) {
	Convey("Given a warmed TimeElastic through FlipFlop", t, func() {
		config := datura.Acquire("time-elastic-flipflop", datura.APPJSON).
			Poke(float64(time.Hour), "config", "halflife").
			Poke(1e-6, "config", "epsilon")
		timeElastic := NewTimeElastic(config)
		artifact := datura.Acquire("test", datura.APPJSON).
			Poke(10, "sample").
			Poke(float64(time.Unix(0, int64(time.Hour)).UnixNano()), "at")

		err := transport.NewFlipFlop(artifact, timeElastic)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)

		artifact.Poke(14, "sample").
			Poke(float64(time.Unix(0, int64(5*time.Hour)).UnixNano()), "at")
		err = transport.NewFlipFlop(artifact, timeElastic)

		So(err, ShouldBeNil)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldBeGreaterThan, 0)
	})
}
