package adaptive

import (
	"io"
	"math"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
	"github.com/theapemachine/nomagique"
)

var timeElasticConfig = datura.Acquire("time-elastic-config", datura.APPJSON).
	Poke("sample", "input").
	Poke(float64(time.Hour), "config", "halflife").
	Poke(1e-6, "config", "epsilon")

func timeElasticWire(sample float64, at time.Time) *datura.Artifact {
	artifact := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", sample)
	artifact.Merge("at", float64(at.UnixNano()))

	return artifact
}

func TestTimeElasticRead(t *testing.T) {
	Convey("Given a TimeElastic", t, func() {
		timeElastic := NewTimeElastic(timeElasticConfig)
		input := timeElasticWire(10, time.Unix(0, 1))
		nomagique.WriteArtifact(timeElastic, input)

		Convey("When Read is called on the first sample", func() {
			frame := make([]byte, 65536)
			readCount, err := timeElastic.Read(frame)
			So(err, ShouldEqual, io.EOF)

			outbound := datura.Acquire("test-out", datura.APPJSON)
			_, err = outbound.Unpack(frame[:readCount])
			So(err, ShouldBeNil)

			rootKey := datura.Peek[string](outbound, "root")

			So(rootKey, ShouldEqual, "output")
			So(datura.Peek[float64](outbound, rootKey, "value"), ShouldEqual, 1)
			So(datura.Peek[bool](outbound, rootKey, "ready"), ShouldBeFalse)
		})

		Convey("When a flat stream advances in time", func() {
			_, _ = nomagique.WriteArtifact(timeElastic, timeElasticWire(10, time.Unix(0, 2)))

			frame := make([]byte, 65536)
			readCount, err := timeElastic.Read(frame)
			So(err, ShouldEqual, io.EOF)

			outbound := datura.Acquire("test-out", datura.APPJSON)
			_, err = outbound.Unpack(frame[:readCount])
			So(err, ShouldBeNil)
			So(datura.Peek[float64](outbound, "output", "value"), ShouldEqual, 1)
			So(datura.Peek[bool](outbound, "output", "ready"), ShouldBeFalse)
		})

		Convey("When a later timestamp regresses", func() {
			_, _ = nomagique.WriteArtifact(timeElastic, timeElasticWire(10, time.Unix(0, 2)))
			_, _ = timeElastic.Read(make([]byte, 65536))
			_, _ = nomagique.WriteArtifact(timeElastic, timeElasticWire(12, time.Unix(0, 1)))

			_, err := timeElastic.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given missing halflife or epsilon", t, func() {
		timeElastic := NewTimeElastic(datura.Acquire("time-elastic-config-missing", datura.APPJSON).Poke("sample", "input"))
		input := timeElasticWire(10, time.Unix(0, 1))
		_, _ = nomagique.WriteArtifact(timeElastic, input)

		Convey("When Read is called", func() {
			_, err := timeElastic.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})
	})

	Convey("Given a non-finite sample", t, func() {
		timeElastic := NewTimeElastic(timeElasticConfig)
		invalid := ScalarWire(datura.Acquire("test", datura.APPJSON), "sample", math.NaN())
		invalid.Merge("at", float64(time.Unix(0, 1).UnixNano()))
		_, _ = nomagique.WriteArtifact(timeElastic, invalid)

		Convey("When Read is called", func() {
			_, err := timeElastic.Read(make([]byte, 65536))

			So(err, ShouldNotBeNil)
		})
	})
}

func TestTimeElasticWrite(t *testing.T) {
	Convey("Given a TimeElastic", t, func() {
		timeElastic := NewTimeElastic(timeElasticConfig)

		Convey("When Write is called", func() {
			input := timeElasticWire(10, time.Unix(0, 1))
			_, err := nomagique.WriteArtifact(timeElastic, input)
			So(err, ShouldBeNil)
		})
	})
}

func TestTimeElasticFlipFlop(t *testing.T) {
	Convey("Given a warmed TimeElastic through FlipFlop", t, func() {
		config := datura.Acquire("time-elastic-flipflop", datura.APPJSON).
			Poke("sample", "input").
			Poke(float64(time.Hour), "config", "halflife").
			Poke(1e-6, "config", "epsilon")
		timeElastic := NewTimeElastic(config)
		artifact := timeElasticWire(10, time.Unix(0, int64(time.Hour)))

		err := nomagique.RoundTripArtifact(artifact, timeElastic)

		So(err, ShouldBeIn, nil, io.EOF)
		So(datura.Peek[float64](artifact, "output", "value"), ShouldEqual, 1)
		So(datura.Peek[bool](artifact, "output", "ready"), ShouldBeFalse)

		second := timeElasticWire(14, time.Unix(0, int64(5*time.Hour)))
		err = nomagique.RoundTripArtifact(second, timeElastic)

		So(err, ShouldBeIn, nil, io.EOF)
		So(datura.Peek[float64](second, "output", "value"), ShouldBeGreaterThan, 0)
		So(datura.Peek[bool](second, "output", "ready"), ShouldBeTrue)
	})
}
