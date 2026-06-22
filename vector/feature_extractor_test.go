package vector

import (
	"bytes"
	"io"
	"math"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/theapemachine/datura"
)

var featureExtractorPayloadFixture = []byte(
	`{"channel":"ticker","type":"update","data":[{"symbol":"BTC/USD","volume":2500,"vwap":100,"last":101,"bid":100.9,"ask":101.1,"change_pct":1.0}]}`,
)

func featureExtractorSchema() *datura.Artifact {
	return datura.Acquire("feature-extractor-test", datura.APPJSON).
		Poke("data", "root").
		Poke([]string{"volume", "vwap", "last", "bid", "ask", "change_pct"}, "inputs")
}

func TestNewFeatureExtractor(t *testing.T) {
	Convey("Given a schema artifact", t, func() {
		schema := datura.Acquire("feature-extractor-schema", datura.APPJSON)

		Convey("When NewFeatureExtractor is called", func() {
			extractor := NewFeatureExtractor(schema)

			Convey("It should retain the schema and initialize transform cache", func() {
				So(extractor, ShouldNotBeNil)
				So(extractor.artifact, ShouldEqual, schema)
				So(extractor.transforms, ShouldNotBeNil)
			})
		})
	})
}

func TestFeatureExtractor_Write(t *testing.T) {
	Convey("Given a feature extractor", t, func() {
		extractor := NewFeatureExtractor(
			datura.Acquire("test", datura.APPJSON).
				Poke("data", "root").
				Poke([]string{"volume"}, "inputs"),
		)

		Convey("And an inbound artifact with a payload", func() {
			inbound := datura.Acquire("test", datura.APPJSON).WithPayload([]byte{1, 2, 3})

			Convey("When the inbound artifact is copied into the extractor", func() {
				_, err := io.Copy(extractor, inbound)

				Convey("Then the inbound wire should be buffered without mutating schema config", func() {
					So(err, ShouldBeNil)
					So(datura.Peek[string](extractor.artifact, "root"), ShouldEqual, "data")
				})
			})
		})
	})
}

func TestFeatureExtractor_Close(t *testing.T) {
	Convey("Given a feature extractor", t, func() {
		extractor := NewFeatureExtractor(datura.Acquire("test", datura.APPJSON))

		Convey("When Close is called", func() {
			err := extractor.Close()

			Convey("It should succeed without error", func() {
				So(err, ShouldBeNil)
			})
		})
	})
}

func TestFeatureExtractor_Read(t *testing.T) {
	Convey("Given a feature extractor with ticker payload", t, func() {
		extractor := NewFeatureExtractor(featureExtractorSchema())

		Convey("When ticker payload is written then read", func() {
			inbound := datura.Acquire("test", datura.APPJSON).WithPayload(featureExtractorPayloadFixture)
			_, err := io.Copy(extractor, inbound)
			So(err, ShouldBeNil)

			wire := make([]byte, 65536)
			n, readErr := extractor.Read(wire)
			So(readErr, ShouldEqual, io.EOF)

			buffer := bytes.NewBuffer(wire[:n])

			Convey("Then the payload should carry root, inputs, and features", func() {
				decoded := datura.Acquire("feature-extractor", datura.APPJSON)
				_, err = decoded.Write(buffer.Bytes())
				So(err, ShouldBeNil)

				So(datura.Peek[string](decoded, "root"), ShouldEqual, "features")
				So(
					datura.Peek[[]string](decoded, "inputs"),
					ShouldResemble,
					[]string{"volume", "vwap", "last", "bid", "ask", "change_pct"},
				)
				So(
					datura.Peek[[]float64](decoded, "features"),
					ShouldResemble,
					[]float64{2500, 100, 101, 100.9, 101.1, 1},
				)

				replayed := datura.Acquire("feature-extractor-replay", datura.APPJSON)
				replayed.WithPayload(decoded.DecryptPayload())

				So(datura.Peek[string](replayed, "root"), ShouldEqual, "features")
				So(
					datura.Peek[[]string](replayed, "inputs"),
					ShouldResemble,
					[]string{"volume", "vwap", "last", "bid", "ask", "change_pct"},
				)

				decoded.Release()
				replayed.Release()
			})
		})
	})

	Convey("Given a non-finite sample field", t, func() {
		extractor := NewFeatureExtractor(
			datura.Acquire("feature-extractor-test", datura.APPJSON).
				Poke("data", "root").
				Poke([]string{"volume"}, "inputs"),
		)
		inbound := datura.Acquire("test", datura.APPJSON).
			Poke("ticker", "channel").
			Poke(math.NaN(), "data", 0, "volume")
		_, err := io.Copy(extractor, inbound)

		So(err, ShouldBeNil)

		Convey("When Read is called", func() {
			_, readErr := extractor.Read(make([]byte, 65536))

			So(readErr, ShouldNotBeNil)
		})
	})
}

func BenchmarkFeatureExtractor_Read(b *testing.B) {
	extractor := NewFeatureExtractor(featureExtractorSchema())
	buffer := make([]byte, 65536)

	b.ReportAllocs()

	for range b.N {
		inbound := datura.Acquire("bench-inbound", datura.APPJSON).
			WithPayload(featureExtractorPayloadFixture)

		if _, err := io.Copy(extractor, inbound); err != nil {
			b.Fatal(err)
		}

		if _, err := extractor.Read(buffer); err != nil && err != io.EOF {
			b.Fatal(err)
		}

		inbound.Release()
	}
}
